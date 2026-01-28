package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Session struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status SessionStatus `json:"status,omitempty"`
}

type SessionStatus struct {
	UserUID       string       `json:"userUID"`
	Provider      string       `json:"provider"`
	IP            string       `json:"ip,omitempty"`
	FingerprintID string       `json:"fingerprintID,omitempty"`
	CreatedAt     metav1.Time  `json:"createdAt"`
	ExpiresAt     *metav1.Time `json:"expiresAt,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SessionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Session `json:"items"`
}
