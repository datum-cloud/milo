package crossclusterutil

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// SetClusterScopedControllerReference assists with entity ownership across
// control planes where the owner in the upstream control plane is cluster
// scoped.
//
// This function will create an "anchor" entity in the API server accessible via
// the provided client to represent the owner that exists in a separate API
// server. This is particularly useful for relying on garbage collection for
// entity destruction versus writing direct teardown logic.
//
// In addition, labels will be added to the controlled entity to identify the
// owner in the upstream control plane. These labels will be used by the
// TypedEnqueueRequestForUpstreamOwner handler to enqueue reconciliations.
func SetClusterScopedControllerReference(
	ctx context.Context,
	c client.Client,
	owner,
	controlled client.Object,
	scheme *runtime.Scheme,
	opts ...controllerutil.OwnerReferenceOption,
) error {

	// TODO(jreese) this prevented setting a cluster scoped owner on GCP
	// resources in the gcp-resource-namespace NS. Think about improving this.

	// infraClusterNamespaceName := InfraClusterNamespaceNameForClusterScopedOwner(owner)

	// if infraClusterNamespaceName != controlled.GetNamespace() {
	// 	return fmt.Errorf("expected controlled object namespace of %q, got %q", infraClusterNamespaceName, controlled.GetNamespace())
	// }

	// For simplicity, we use a ConfigMap for an anchor. This may change to a
	// separate type in the future if ConfigMap bloat causes an issue in caches.

	gvk, err := apiutil.GVKForObject(owner.(runtime.Object), scheme)
	if err != nil {
		return err
	}

	anchorLabels := map[string]string{
		UpstreamOwnerGroupLabel: gvk.Group,
		UpstreamOwnerKindLabel:  gvk.Kind,
		UpstreamOwnerNameLabel:  owner.GetName(),
		EntityPurposeLabel:      "anchor",
	}

	listOpts := []client.ListOption{
		client.InNamespace(controlled.GetNamespace()),
		client.MatchingLabels(anchorLabels),
	}

	var configMaps corev1.ConfigMapList
	if err := c.List(ctx, &configMaps, listOpts...); err != nil {
		return fmt.Errorf("failed listing configmaps: %w", err)
	}

	var anchorConfigMap corev1.ConfigMap
	if len(configMaps.Items) == 0 {
		// create configmap
		anchorConfigMap = corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    controlled.GetNamespace(),
				GenerateName: fmt.Sprintf("anchor-%s-", owner.GetName()),
				Labels:       anchorLabels,
			},
		}

		if err := c.Create(ctx, &anchorConfigMap); err != nil {
			return fmt.Errorf("failed creating anchor configmap: %w", err)
		}

	} else if len(configMaps.Items) > 1 {
		// Never expect this to happen, but better to stop doing any work if it does.
		return fmt.Errorf("expected 1 anchor configmap, got: %d", len(configMaps.Items))
	} else {
		anchorConfigMap = configMaps.Items[0]
	}

	if err := controllerutil.SetOwnerReference(&anchorConfigMap, controlled, scheme, opts...); err != nil {
		return fmt.Errorf("failed setting anchor owner reference: %w", err)
	}

	labels := controlled.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[UpstreamOwnerGroupLabel] = anchorLabels[UpstreamOwnerGroupLabel]
	labels[UpstreamOwnerKindLabel] = anchorLabels[UpstreamOwnerKindLabel]
	labels[UpstreamOwnerNameLabel] = anchorLabels[UpstreamOwnerNameLabel]
	controlled.SetLabels(labels)

	return nil
}

// DeleteAnchorForObject will delete the anchor configmap associated with the
// provided owner, which will help drive GC of other entities.
func DeleteAnchorForObject(
	ctx context.Context,
	upstreamClient client.Client,
	infraClusterClient client.Client,
	owner client.Object,
) error {

	infraClusterNamespaceName := InfraClusterNamespaceNameForClusterScopedOwner(owner)

	listOpts := []client.ListOption{
		client.InNamespace(infraClusterNamespaceName),
		client.MatchingLabels{
			UpstreamOwnerGroupLabel: owner.GetObjectKind().GroupVersionKind().Group,
			UpstreamOwnerKindLabel:  owner.GetObjectKind().GroupVersionKind().Kind,
			UpstreamOwnerNameLabel:  owner.GetName(),
			EntityPurposeLabel:      "anchor",
		},
	}

	var configMaps corev1.ConfigMapList
	if err := infraClusterClient.List(ctx, &configMaps, listOpts...); err != nil {
		return fmt.Errorf("failed listing configmaps: %w", err)
	}

	if len(configMaps.Items) == 0 {
		return nil
	}

	if len(configMaps.Items) > 1 {
		// Never expect this to happen, but better to stop doing any work if it does.
		return fmt.Errorf("expected 1 anchor configmap, got: %d", len(configMaps.Items))
	}

	return infraClusterClient.Delete(ctx, &configMaps.Items[0])
}
