package v1alpha1

// ScopedRoleReference defines a reference to another Role, scoped by namespace.
// This is used for role inheritance where one role needs to reference another
// role to inherit its permissions. The reference includes both name and optional
// namespace for cross-namespace role inheritance.
//
// Example usage in role inheritance:
//   inheritedRoles:
//   - name: viewer-role        # references viewer-role in same namespace
//   - name: admin-base
//     namespace: system        # references admin-base in system namespace
//
// +k8s:deepcopy-gen=true
// +kubebuilder:object:generate=true
type ScopedRoleReference struct {
	// Name of the referenced Role. This must match the metadata.name of an
	// existing Role resource that contains the permissions to be inherited.
	//
	// Example: "workload-viewer"
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the referenced Role. If not specified, it defaults to the
	// namespace of the resource containing this reference, enabling same-namespace
	// role inheritance without explicit namespace specification.
	//
	// For cross-namespace inheritance, this field must be explicitly set to
	// the namespace containing the target role.
	//
	// Example: "system" (for system-wide roles) or "shared-roles"
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`
}
