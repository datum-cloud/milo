package crossclusterutil

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func InfraClusterNamespaceNameForClusterScopedOwner(owner client.Object) string {
	return fmt.Sprintf("project-%s", owner.GetUID())
}
