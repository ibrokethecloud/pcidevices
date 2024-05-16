package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

type USBDeviceClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   USBDeviceClaimSpec   `json:"spec,omitempty"`
	Status USBDeviceClaimStatus `json:"status,omitempty"`
}

type USBDeviceClaimSpec struct {
}

type USBDeviceClaimStatus struct {
	NodeName   string `json:"nodeName"`
	PCIAddress string `json:"pciAddress"`
	ClaimedBy  string `json:"claimedBy"`
}
