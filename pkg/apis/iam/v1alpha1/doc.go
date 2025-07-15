// Package v1alpha1 contains API Schema definitions for the iam v1alpha1 API group
// +k8s:deepcopy-gen=package,register
// +groupName=iam.miloapis.com
package v1alpha1

const (
	// ParentNameExtraKey is the key used to store the parent resource name from a field selector
	// in the user's authentication extra data.
	ParentNameExtraKey = "iam.miloapis.com/parent-name"
	// ParentKindExtraKey is the key used to store the parent resource type from a field selector
	// in the user's authentication extra data.
	ParentKindExtraKey = "iam.miloapis.com/parent-type"
	// ParentAPIGroupExtraKey is the key used to store the parent resource API group from a field selector
	// in the user's authentication extra data.
	ParentAPIGroupExtraKey = "iam.miloapis.com/parent-api-group"
)
