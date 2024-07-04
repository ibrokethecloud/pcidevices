package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

type USBDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              USBDeviceSpec   `json:"spec"`
	Status            USBDeviceStatus `json:"status,omitempty"`
}

type USBDeviceStatus struct {
	VendorID    string `json:"vendorID"`
	ProductID   string `json:"productID"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

type USBDeviceSpec struct {
	ResourceName string `json:"resourceName"`
	NodeName     string `json:"nodeName"`
	DevicePath   string `json:"devicePath"`
	PCIAddress   string `json:"pciAddress"`
}
