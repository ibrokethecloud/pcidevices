package usbdevice

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/deviceplugins"
	ctldevicerv1beta1 "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
	ctlkubevirtv1 "github.com/harvester/pcidevices/pkg/generated/controllers/kubevirt.io/v1"
)

type DevClaimHandler struct {
	usbClaimClient       ctldevicerv1beta1.USBDeviceClaimClient
	usbClient            ctldevicerv1beta1.USBDeviceClient
	virtClient           ctlkubevirtv1.KubeVirtClient
	lock                 *sync.Mutex
	usbDeviceCache       ctldevicerv1beta1.USBDeviceCache
	managedDevicePlugins map[string]*deviceplugins.USBDevicePlugin
}

func NewClaimHandler(
	usbDeviceCache ctldevicerv1beta1.USBDeviceCache,
	usbClaimClient ctldevicerv1beta1.USBDeviceClaimClient,
	usbClient ctldevicerv1beta1.USBDeviceClient,
	virtClient ctlkubevirtv1.KubeVirtClient,
) *DevClaimHandler {
	return &DevClaimHandler{
		usbDeviceCache:       usbDeviceCache,
		usbClaimClient:       usbClaimClient,
		usbClient:            usbClient,
		virtClient:           virtClient,
		lock:                 &sync.Mutex{},
		managedDevicePlugins: make(map[string]*deviceplugins.USBDevicePlugin),
	}
}

func (h *DevClaimHandler) OnUSBDeviceClaimChanged(_ string, usbDeviceClaim *v1beta1.USBDeviceClaim) (*v1beta1.USBDeviceClaim, error) {
	if usbDeviceClaim == nil || usbDeviceClaim.DeletionTimestamp != nil {
		return usbDeviceClaim, nil
	}

	if usbDeviceClaim.OwnerReferences == nil {
		err := fmt.Errorf("usb device claim %s has no owner reference", usbDeviceClaim.Name)
		logrus.Error(err)
		return usbDeviceClaim, err
	}

	usbDevice, err := h.usbDeviceCache.Get(usbDeviceClaim.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Errorf("usb device %s not found", usbDeviceClaim.Name)
			return usbDeviceClaim, nil
		}
		return usbDeviceClaim, err
	}

	h.lock.Lock()
	defer h.lock.Unlock()

	virt, err := h.virtClient.Get(KubeVirtNamespace, KubeVirtResource, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("failed to get kubevirt: %v", err)
		return usbDeviceClaim, err
	}

	// update kubevirt CR to ensure the usb device state on host matches the kubevirt description
	_, err = h.updateKubeVirt(virt, usbDevice)
	if err != nil {
		logrus.Errorf("failed to update kubevirt: %v", err)
		return usbDeviceClaim, err
	}

	devicePlugin, ok := h.managedDevicePlugins[usbDeviceClaim.Name]

	// if plugin exists check if it is started
	// and start it if needed
	if !ok {
		devicePlugin = deviceplugins.NewUSBDevicePlugin(*usbDevice)
		h.managedDevicePlugins[usbDeviceClaim.Name] = devicePlugin
	}

	if !devicePlugin.GetInitialized() {
		devicePlugin.StartDevicePlugin()
	}

	// update usbDeviceStatus to reflect correct status
	if !usbDevice.Status.Enabled {
		usbDeviceCp := usbDevice.DeepCopy()
		usbDeviceCp.Status.Enabled = true
		if _, err = h.usbClient.UpdateStatus(usbDeviceCp); err != nil {
			logrus.Errorf("failed to enable usb device %s status: %v", usbDeviceCp.Name, err)
			return usbDeviceClaim, err
		}
	}

	// just sync usb device pci address to usb device claim
	usbDeviceClaimCp := usbDeviceClaim.DeepCopy()
	usbDeviceClaimCp.Status.PCIAddress = usbDevice.Spec.PCIAddress
	usbDeviceClaimCp.Status.NodeName = usbDevice.Spec.NodeName

	return h.usbClaimClient.UpdateStatus(usbDeviceClaimCp)
}

func (h *DevClaimHandler) OnRemove(_ string, claim *v1beta1.USBDeviceClaim) (*v1beta1.USBDeviceClaim, error) {
	if claim == nil || claim.DeletionTimestamp == nil {
		return claim, nil
	}

	usbDevice, err := h.usbDeviceCache.Get(claim.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			fmt.Println("usbClient device not found")
			return claim, nil
		}
		return claim, err
	}

	h.lock.Lock()
	defer h.lock.Unlock()

	virt, err := h.virtClient.Get(KubeVirtNamespace, KubeVirtResource, metav1.GetOptions{})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	virtDp := virt.DeepCopy()

	if len(virtDp.Spec.Configuration.PermittedHostDevices.USB) == 0 {
		return claim, nil
	}

	usbs := virtDp.Spec.Configuration.PermittedHostDevices.USB

	// split target one if usb.ResourceName == usbDevice.Name

	for i, usb := range usbs {
		if usb.ResourceName == usbDevice.Spec.ResourceName {
			usbs = append(usbs[:i], usbs[i+1:]...)
			break
		}
	}

	virtDp.Spec.Configuration.PermittedHostDevices.USB = usbs

	if !reflect.DeepEqual(virt.Spec.Configuration.PermittedHostDevices.USB, virtDp.Spec.Configuration.PermittedHostDevices.USB) {
		if _, err := h.virtClient.Update(virtDp); err != nil {
			return claim, nil
		}
	}

	if plugin, ok := h.managedDevicePlugins[claim.Name]; ok {
		if err := plugin.StopDevicePlugin(); err != nil {
			return claim, fmt.Errorf("error stopping device plugin %s: %v", plugin.GetResourceName(), err)
		}
		delete(h.managedDevicePlugins, claim.Name)
	}

	usbDeviceCp := usbDevice.DeepCopy()
	usbDeviceCp.Status.Enabled = false
	if _, err = h.usbClient.UpdateStatus(usbDeviceCp); err != nil {
		logrus.Errorf("failed to disable usb device %s status: %v", usbDeviceCp.Name, err)
		return claim, err
	}

	return claim, nil
}

func (h *DevClaimHandler) updateKubeVirt(virt *kubevirtv1.KubeVirt, usbDevice *v1beta1.USBDevice) (*kubevirtv1.KubeVirt, error) {
	virtDp := virt.DeepCopy()

	if virtDp.Spec.Configuration.PermittedHostDevices == nil {
		virtDp.Spec.Configuration.PermittedHostDevices = &kubevirtv1.PermittedHostDevices{
			USB: make([]kubevirtv1.USBHostDevice, 0),
		}
	}

	usbs := virtDp.Spec.Configuration.PermittedHostDevices.USB

	expectedUSBDeviceEntry := kubevirtv1.USBHostDevice{
		ResourceName: usbDevice.Spec.ResourceName,
		Selectors: []kubevirtv1.USBSelector{
			{
				Vendor:  usbDevice.Status.VendorID,
				Product: usbDevice.Status.ProductID,
			},
		},
		ExternalResourceProvider: true,
	}

	var found bool
	var index int
	// check if the usb device is already added
	for i, usb := range usbs {
		// if resource name matches
		if usb.ResourceName == usbDevice.Spec.ResourceName {
			found = true
			index = i
		}
	}

	// if a resource name is found but vendor/product id may have changed due to device attachment changing
	// then update the usb entry in place
	if found {
		if !reflect.DeepEqual(usbs[index], expectedUSBDeviceEntry) {
			usbs[index] = expectedUSBDeviceEntry
		}
	} else {
		usbs = append(usbs, expectedUSBDeviceEntry)
	}

	virtDp.Spec.Configuration.PermittedHostDevices.USB = usbs
	if virt.Spec.Configuration.PermittedHostDevices == nil || !reflect.DeepEqual(virt.Spec.Configuration.PermittedHostDevices.USB, virtDp.Spec.Configuration.PermittedHostDevices.USB) {
		newVirt, err := h.virtClient.Update(virtDp)
		if err != nil {
			logrus.Errorf("failed to update kubevirt: %v", err)
			return virt, err
		}

		return newVirt, nil
	}

	return virt, nil
}
