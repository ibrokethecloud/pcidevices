package pcideviceclaim

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/deviceplugins"
	v1beta1gen "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/u-root/u-root/pkg/kmodule"
	kubevirtv1 "kubevirt.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/client-go/kubecli"
)

const (
	reconcilePeriod = time.Minute * 1
	vfioPCIDriver   = "vfio-pci"
	DefaultNS       = "harvester-system"
	KubevirtCR      = "kubevirt"
)

type Controller struct {
	PCIDeviceClaims v1beta1gen.PCIDeviceClaimController
}

type Handler struct {
	pdcClient     v1beta1gen.PCIDeviceClaimClient
	pdClient      v1beta1gen.PCIDeviceClient
	virtClient    kubecli.KubevirtClient
	nodeName      string
	devicePlugins map[string]*deviceplugins.PCIDevicePlugin
}

func Register(
	ctx context.Context,
	pdcClient v1beta1gen.PCIDeviceClaimController,
	pdClient v1beta1gen.PCIDeviceController,
) error {
	logrus.Info("Registering PCI Device Claims controller")
	nodename := os.Getenv("NODE_NAME")
	clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
	if err != nil {
		msg := fmt.Sprintf("cannot obtain KubeVirt client: %v", err)
		return errors.New(msg)
	}

	handler := &Handler{
		pdcClient:     pdcClient,
		pdClient:      pdClient,
		nodeName:      nodename,
		virtClient:    virtClient,
		devicePlugins: make(map[string]*deviceplugins.PCIDevicePlugin),
	}

	pdcClient.OnRemove(ctx, "PCIDeviceClaimOnRemove", handler.OnRemove)
	pdcClient.OnChange(ctx, "PCIDeviceClaimReconcile", handler.reconcilePCIDeviceClaims)
	err = handler.rebindAfterReboot()
	if err != nil {
		return err
	}
	err = handler.unbindOrphanedPCIDevices()
	if err != nil {
		return err
	}
	// Load VFIO drivers when controller starts instead of repeatedly in the reconcile loop
	loadVfioDrivers()
	return nil
}

// When a PCIDeviceClaim is removed, we need to unbind the device from the vfio-pci driver
func (h Handler) OnRemove(name string, pdc *v1beta1.PCIDeviceClaim) (*v1beta1.PCIDeviceClaim, error) {
	if pdc == nil || pdc.DeletionTimestamp == nil || pdc.Spec.NodeName != h.nodeName {
		return pdc, nil
	}

	// Get PCIDevice for the PCIDeviceClaim
	pd, err := h.getPCIDeviceForClaim(pdc)
	if err != nil {
		logrus.Errorf("Error getting claim's device: %s", err)
		return pdc, err
	}


	// Get PCIDevice for the PCIDeviceClaim
	pd, err := h.getPCIDeviceForClaim(pdc)
	if err != nil {
		logrus.Errorf("Error getting claim's device: %s", err)
		return pdc, err
	}

	// Disable PCI Passthrough by unbinding from the vfio-pci device driver
	newPdc, err := h.attemptToDisablePassthrough(pd, pdc)
	if err != nil {
		return newPdc, err
	}

	// Find the DevicePlugin
	resourceName := pd.Status.ResourceName
	dp := deviceplugins.Find(
		resourceName,
		h.devicePlugins,
	)
	if dp != nil {
		err = dp.RemoveDevice(pd, pdc)
		if err != nil {
			return pdc, err
		}
		// Check if that was the last device, and then shut down the dp
		time.Sleep(5 * time.Second)
		if dp.GetCount() == 0 {
			err := dp.Stop()
			if err != nil {
				return pdc, err
			}
			delete(h.devicePlugins, resourceName)
		}
	}
	return pdc, nil
}

func loadVfioDrivers() {
	for _, driver := range []string{"vfio-pci", "vfio_iommu_type1"} {
		logrus.Infof("Loading driver %s", driver)
		if err := kmodule.Probe(driver, ""); err != nil {
			logrus.Error(err)
		}
	}
}

func bindDeviceToVFIOPCIDriver(pd *v1beta1.PCIDevice) error {
	vendorId := pd.Status.VendorId
	deviceId := pd.Status.DeviceId
	var id string = fmt.Sprintf("%s %s", vendorId, deviceId)
	logrus.Infof("Binding device %s [%s] to vfio-pci", pd.Name, id)

	file, err := os.OpenFile("/sys/bus/pci/drivers/vfio-pci/new_id", os.O_WRONLY, 0400)
	if err != nil {
		logrus.Errorf("Error opening new_id file: %s", err)
		return err
	}
	_, err = file.WriteString(id)
	if err != nil {
		logrus.Errorf("Error writing to new_id file: %s", err)
		file.Close()
		return err
	}
	file.Close()
	return nil
}

// Enabling passthrough for a PCI Device requires two steps:
// 1. Bind the device to the vfio-pci driver in the host
// 2. Add device to DevicePlugin so KubeVirt will recognize it
func (h Handler) enablePassthrough(pd *v1beta1.PCIDevice) error {
	err := bindDeviceToVFIOPCIDriver(pd)
	if err != nil {
		return err
	}
	return nil
}

func (h Handler) disablePassthrough(pd *v1beta1.PCIDevice) error {
	errDriver := unbindDeviceFromDriver(pd.Status.Address, vfioPCIDriver)
	if errDriver != nil {
		return errDriver
	}
	if errDriver != nil {
		msg := fmt.Sprintf("failed unbinding driver: (%s)", errDriver)
		return errors.New(msg)
	}
	return nil
}

// This function unbinds the device with PCI Address addr from the given driver
// NOTE: this function assumes that addr is on THIS NODE, only call for PCI addrs on this node
func unbindDeviceFromDriver(addr string, driver string) error {
	driverPath := fmt.Sprintf("/sys/bus/pci/drivers/%s", driver)
	// Check if device at addr is already bound to driver
	_, err := os.Stat(fmt.Sprintf("%s/%s", driverPath, addr))
	if err != nil {
		logrus.Errorf("Device at address %s is not bound to driver %s", addr, driver)
		return nil
	}
	path := fmt.Sprintf("%s/unbind", driverPath)
	file, err := os.OpenFile(path, os.O_WRONLY, 0400)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(addr)
	if err != nil {
		return err
	}
	return nil
}

func pciDeviceIsClaimed(pd *v1beta1.PCIDevice, pdcs *v1beta1.PCIDeviceClaimList) bool {
	for _, pdc := range pdcs.Items {
		if pdc.OwnerReferences == nil {
			return false
		}
		if pdc.OwnerReferences[0].Name == pd.Name {
			return true
		}
	}
	return false
}

// A PCI Device is considered orphaned if it is bound to vfio-pci,
// but has no PCIDeviceClaim. The assumption is that this controller
// will manage all PCI passthrough, and consider orphaned devices invalid
func getOrphanedPCIDevices(
	nodename string,
	pdcs *v1beta1.PCIDeviceClaimList,
	pds *v1beta1.PCIDeviceList,
) (*v1beta1.PCIDeviceList, error) {
	pdsOrphaned := v1beta1.PCIDeviceList{}
	for _, pd := range pds.Items {
		isVfioPci := pd.Status.KernelDriverInUse == "vfio-pci"
		isOnThisNode := nodename == pd.Status.NodeName
		if isVfioPci && isOnThisNode && !pciDeviceIsClaimed(&pd, pdcs) {
			pdsOrphaned.Items = append(pdsOrphaned.Items, *pd.DeepCopy())
		}
	}
	return &pdsOrphaned, nil
}

// After reboot, the PCIDeviceClaim will be there but the PCIDevice won't be bound to vfio-pci
func (h Handler) rebindAfterReboot() error {
	logrus.Infof("Rebinding after reboot on node: %s", h.nodeName)
	pdcs, err := h.pdcClient.List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Error getting claims: %s", err)
		return err
	}
	var errUpdateStatus error = nil
	for _, pdc := range pdcs.Items {
		if pdc.Spec.NodeName != h.nodeName {
			continue
		}
		// Get PCIDevice for the PCIDeviceClaim
		name := pdc.OwnerReferences[0].Name
		pd, err := h.pdClient.Get(name, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Error getting claim's device: %s", err)
			continue
		}

		if pd.Status.KernelDriverInUse == "vfio-pci" {
			logrus.Infof("PCIDevice %s is already bound to vfio-pci, skipping", pd.Name)
			continue
		}

		logrus.Infof("Passthrough disabled for device %s", pd.Name)
		pdcCopy := pdc.DeepCopy()

		// Try to unbind from existing driver, if applicable
		err = unbindDeviceFromDriver(pd.Status.Address, pd.Status.KernelDriverInUse)
		if err != nil {
			pdcCopy.Status.PassthroughEnabled = true
			logrus.Errorf("Error unbinding device after reboot: %s", err)
		} else {
			pdcCopy.Status.PassthroughEnabled = false
		}

		// Enable Passthrough on the device
		err = h.enablePassthrough(pd)
		if err != nil {
			logrus.Errorf("Error rebinding device after reboot: %s", err)
			pdcCopy.Status.PassthroughEnabled = false

		} else {
			pdcCopy.Status.PassthroughEnabled = true
		}
		_, err = h.pdcClient.UpdateStatus(pdcCopy)
		if err != nil {
			logrus.Errorf("Failed to update PCIDeviceClaim status for %s: %s", pdc.Name, err)
			errUpdateStatus = err
		}
	}
	return errUpdateStatus
}

func (h Handler) reconcilePCIDeviceClaims(name string, pdc *v1beta1.PCIDeviceClaim) (*v1beta1.PCIDeviceClaim, error) {

	if pdc == nil || pdc.DeletionTimestamp != nil || (pdc.Spec.NodeName != h.nodeName) {
		return pdc, nil
	}

	// Get the PCIDevice object for the PCIDeviceClaim
	pd, err := h.getPCIDeviceForClaim(pdc)
	if pd == nil {
		return pdc, err
	}

	// Find the DevicePlugin
	resourceName := pd.Status.ResourceName
	dp := deviceplugins.Find(
		resourceName,
		h.devicePlugins,
	)

	if err := h.permitHostDeviceInKubeVirt(pd); err != nil {
		return pdc, fmt.Errorf("error updating kubevirt CR: %v", err)
	}

	// If passthrough is already enabled and there's a deviceplugin, then we can return
	if pdc.Status.PassthroughEnabled && (dp != nil){
		return pdc, nil
	}

	// Enable PCI Passthrough on the device by binding it to vfio-pci driver
	newPdc, err := h.attemptToEnablePassthrough(pd, pdc)

	// If the DevicePlugin can't be found, then create it
	if dp == nil {
		logrus.Infof("Creating DevicePlugin: %s", resourceName)
		pds := []*v1beta1.PCIDevice{pd}
		dp = deviceplugins.Create(resourceName, pdc, pds)
		h.devicePlugins[resourceName] = dp
		// Start the DevicePlugin
		if newPdc.Status.PassthroughEnabled && !dp.Started() {
			err = h.startDevicePlugin(pd, newPdc, dp)
			if err != nil {
				return pdc, err
			}
		}
	} else {
		// Add the Device to the DevicePlugin
		dp.AddDevice(pd, pdc)
	}
	return newPdc, err
}

func (h Handler) permitHostDeviceInKubeVirt(pd *v1beta1.PCIDevice) error {
	logrus.Infof("Adding %s to KubeVirt list of permitted devices", pd.Name)
	kv, err := h.virtClient.KubeVirt(DefaultNS).Get(KubevirtCR, &v1.GetOptions{})
	if err != nil {
		msg := fmt.Sprintf("cannot obtain KubeVirt CR: %v", err)
		return errors.New(msg)
	}
	kvCopy := kv.DeepCopy()
	if kv.Spec.Configuration.PermittedHostDevices == nil {
		kvCopy.Spec.Configuration.PermittedHostDevices = &kubevirtv1.PermittedHostDevices{
			PciHostDevices: []kubevirtv1.PciHostDevice{},
		}
	}
	permittedPCIDevices := kvCopy.Spec.Configuration.PermittedHostDevices.PciHostDevices
	resourceName := pd.Status.ResourceName
	// check if device is currently permitted
	var devPermitted bool = false
	for _, permittedPCIDev := range permittedPCIDevices {
		if permittedPCIDev.ResourceName == resourceName {
			devPermitted = true
			break
		}
	}
	if !devPermitted {
		vendorId := pd.Status.VendorId
		deviceId := pd.Status.DeviceId
		devToPermit := kubevirtv1.PciHostDevice{
			PCIVendorSelector:        fmt.Sprintf("%s:%s", vendorId, deviceId),
			ResourceName:             resourceName,
			ExternalResourceProvider: true,
		}
		kvCopy.Spec.Configuration.PermittedHostDevices.PciHostDevices = append(permittedPCIDevices, devToPermit)
		_, err := h.virtClient.KubeVirt(DefaultNS).Update(kvCopy)
		if err != nil {
			msg := fmt.Sprintf("Failed to update kubevirt CR: %s", err)
			return errors.New(msg)
		}
	}
	return nil
}

func (h Handler) getPCIDeviceForClaim(pdc *v1beta1.PCIDeviceClaim) (*v1beta1.PCIDevice, error) {
	// Get PCIDevice for the PCIDeviceClaim
	if pdc.OwnerReferences == nil {
		msg := fmt.Sprintf("Cannot find PCIDevice that owns %s", pdc.Name)
		return nil, errors.New(msg)
	}
	name := pdc.OwnerReferences[0].Name
	pd, err := h.pdClient.Get(name, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("error getting claim's device: %s", err)
		return nil, err
	}
	return pd, nil
}

func (h Handler) startDevicePlugin(
	pd *v1beta1.PCIDevice,
	pdc *v1beta1.PCIDeviceClaim,
	dp *deviceplugins.PCIDevicePlugin,
) error {
	if dp.Started() {
		return nil
	}
	// Start the plugin
	stop := make(chan struct{})
	go func() {
		err := dp.Start(stop)
		if err != nil {
			logrus.Errorf("error starting %s device plugin: %s", dp.GetDeviceName(), err)
		}
		// TODO: test if deleting this stops the DevicePlugin
		<-stop
	}()
	dp.SetStarted(stop)
	return nil
}

func (h Handler) attemptToEnablePassthrough(pd *v1beta1.PCIDevice, pdc *v1beta1.PCIDeviceClaim) (*v1beta1.PCIDeviceClaim, error) {
	pdcCopy := pdc.DeepCopy()
	pdcCopy.Status.KernelDriverToUnbind = pd.Status.KernelDriverInUse
	vfioDriverEnabled := pd.Status.KernelDriverInUse == vfioPCIDriver // it is possible that device was enabled however pcideviceclaim status updated failed

	if !vfioDriverEnabled {
		logrus.Infof("Enabling passthrough for PDC: %s", pdc.Name)
		// Only unbind from driver is a driver is currently in use
		if strings.TrimSpace(pd.Status.KernelDriverInUse) != "" {
			err := unbindDeviceFromDriver(pd.Status.Address, pd.Status.KernelDriverInUse)
			if err != nil {
				pdcCopy.Status.PassthroughEnabled = false
			}
		}
		// Enable PCI Passthrough by binding the device to the vfio-pci driver
		err := h.enablePassthrough(pd, pdc)
		if err != nil {
			return pdc, err
		}
	}

	if !pdcCopy.Status.PassthroughEnabled {
		pdcCopy.Status.PassthroughEnabled = true
		return h.pdcClient.UpdateStatus(pdcCopy)
	}

	return pdc, nil

}

func (h Handler) attemptToDisablePassthrough(pd *v1beta1.PCIDevice, pdc *v1beta1.PCIDeviceClaim) error {
	logrus.Infof("Attempting to disable passthrough for %s", pdc.Name)
	pdcCopy := pdc.DeepCopy()
	pdcCopy.Status.KernelDriverToUnbind = pd.Status.KernelDriverInUse
	if pd.Status.KernelDriverInUse == vfioPCIDriver {
		pdcCopy.Status.PassthroughEnabled = true
		// Only unbind from driver is a driver is currently bound to vfio
		if strings.TrimSpace(pd.Status.KernelDriverInUse) == vfioPCIDriver {
			err := unbindDeviceFromDriver(pd.Status.Address, vfioPCIDriver)
			if err != nil {
				return err
			}
		}
		// Disable PCI Passthrough by binding the device to the vfio-pci driver
		err := h.disablePassthrough(pd)
		if err != nil {
			pdcCopy.Status.PassthroughEnabled = true
		} else {
			pdcCopy.Status.PassthroughEnabled = false
		}
	}
	newPdc, err := h.pdcClient.UpdateStatus(pdcCopy)
	if err != nil {
		logrus.Errorf("Error updating status for %s: %s", pdc.Name, err)
		return err
	}
	return nil
}

func (h Handler) unbindOrphanedPCIDevices() error {
	pdcs, err := h.pdcClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	pds, err := h.pdClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	orphanedPCIDevices, err := getOrphanedPCIDevices(h.nodeName, pdcs, pds)
	if err != nil {
		return err
	}
	for _, pd := range orphanedPCIDevices.Items {
		unbindDeviceFromDriver(pd.Status.Address, vfioPCIDriver)
	}
	return nil
}
