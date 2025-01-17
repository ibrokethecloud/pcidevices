package v1beta1

import (
	"fmt"
	"strings"

	"github.com/harvester/pcidevices/pkg/lspci"
	"github.com/sirupsen/logrus"
	"github.com/u-root/u-root/pkg/pci"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// PCIDevice is the Schema for the pcidevices API
type PCIDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PCIDeviceSpec   `json:"spec,omitempty"`
	Status PCIDeviceStatus `json:"status,omitempty"`
}

// PCIDeviceStatus defines the observed state of PCIDevice
type PCIDeviceStatus struct {
	Address           string   `json:"address"`
	VendorId          int      `json:"vendorId"`
	DeviceId          int      `json:"deviceId"`
	NodeName          string   `json:"nodeName"`
	Description       string   `json:"description"`
	KernelDriverInUse string   `json:"kernelDriverInUse,omitempty"`
	KernelModules     []string `json:"kernelModules"`
}

func (status *PCIDeviceStatus) Update(dev *pci.PCI, hostname string) {
	lspciOutput, err := lspci.GetLspciOuptut(dev.Addr)
	if err != nil {
		logrus.Error(err)
	}
	driver, err := lspci.ExtractCurrentPCIDriver(lspciOutput)
	if err != nil {
		logrus.Error(err)
		// Continue and update the object even if driver is not found
	}
	status.Address = dev.Addr
	status.VendorId = int(dev.Vendor)
	status.DeviceId = int(dev.Device)
	status.Description = dev.DeviceName
	status.KernelDriverInUse = driver
	status.NodeName = hostname

	modules, err := lspci.ExtractKernelModules(lspciOutput)
	if err != nil {
		logrus.Error(err)
		// Continue and update the object even if modules are not found
	}
	status.KernelModules = modules
}

type PCIDeviceSpec struct {
}

func PCIDeviceNameForHostname(dev *pci.PCI, hostname string) string {
	vendorName := strings.ToLower(
		strings.Split(dev.VendorName, " ")[0],
	)
	addrDNSsafe := strings.ReplaceAll(strings.ReplaceAll(dev.Addr, ":", ""), ".", "")
	return fmt.Sprintf(
		"%s-%s-%x-%x-%s",
		hostname,
		vendorName,
		dev.Vendor,
		dev.Device,
		addrDNSsafe,
	)
}

func NewPCIDeviceForHostname(dev *pci.PCI, hostname string) PCIDevice {
	name := PCIDeviceNameForHostname(dev, hostname)
	pciDevice := PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: PCIDeviceStatus{
			Address:     dev.Addr,
			VendorId:    int(dev.Vendor), // upcasting a uint16 to an int is safe
			DeviceId:    int(dev.Device),
			NodeName:    hostname,
			Description: dev.DeviceName,
		},
	}
	return pciDevice
}
