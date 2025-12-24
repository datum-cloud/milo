package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type IdentityProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status IdentityProviderStatus `json:"status,omitempty"`
}

type IdentityProviderStatus struct {
	UserUID      string `json:"userUID"`
	ProviderID   string `json:"providerID"`
	ProviderName string `json:"providerName"`
	Username     string `json:"username"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type IdentityProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IdentityProvider `json:"items"`
}
