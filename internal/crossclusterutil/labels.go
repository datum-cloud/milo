package crossclusterutil

const (
	LabelNamespace = "resourcemanager.miloapis.com"

	ProjectNameLabel = LabelNamespace + "/project-name"
	OwnerNameLabel   = LabelNamespace + "/owner-name"

	UpstreamOwnerGroupLabel     = LabelNamespace + "/upstream-group"
	UpstreamOwnerKindLabel      = LabelNamespace + "/upstream-kind"
	UpstreamOwnerNameLabel      = LabelNamespace + "/upstream-name"
	UpstreamOwnerNamespaceLabel = LabelNamespace + "/upstream-namespace"

	EntityPurposeLabel = LabelNamespace + "/entity-purpose"
)
