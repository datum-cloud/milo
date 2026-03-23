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

// SessionStatus contains session metadata exposed for display and management.
// All fields except those required for identity are optional and populated by the authentication provider.
type SessionStatus struct {
	// UserUID is the unique identifier of the user who owns this session.
	UserUID string `json:"userUID"`

	// Provider is the authentication provider for this session (e.g. "zitadel").
	Provider string `json:"provider"`

	// IP is the client IP address associated with the session, if known.
	IP string `json:"ip,omitempty"`

	// FingerprintID is an optional device or client fingerprint from the provider.
	FingerprintID string `json:"fingerprintID,omitempty"`

	// CreatedAt is when the session was created.
	CreatedAt metav1.Time `json:"createdAt"`

	// Location is a human-readable geographic label for the client (e.g. "Bristol, United Kingdom"),
	// typically derived from GeoIP by the provider.
	Location string `json:"location,omitempty"`

	// Browser is the detected client browser or app name (e.g. "Safari", "Chrome").
	Browser string `json:"browser,omitempty"`

	// OS is the detected operating system (e.g. "macOS", "Windows").
	OS string `json:"os,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SessionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Session `json:"items"`
}
