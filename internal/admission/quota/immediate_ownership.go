package quota

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// createResourceClaimWithImmediateOwnership creates a ResourceClaim with immediate owner reference
// when the target resource exists in the admission context.
func (p *ClaimCreationPlugin) createResourceClaimWithImmediateOwnership(ctx context.Context, policy *quotav1alpha1.ClaimCreationPolicy, evalContext *EvaluationContext) (string, string, error) {
	// Since we're in the admission context, the target resource is being created right now
	// We can immediately set the owner reference using the resource being admitted

	// Render the ResourceClaim from the policy template
	spec, err := p.templateEngine.RenderResourceClaim(ctx, policy.Spec.ResourceClaimTemplate, evalContext, p.policyEngine)
	if err != nil {
		return "", "", fmt.Errorf("failed to render ResourceClaim spec: %w", err)
	}

	// Generate ResourceClaim name prefix for GenerateName
	claimNamePrefix := p.generateResourceClaimNamePrefix(evalContext)

	// Determine namespace
	namespace := quotav1alpha1.MiloSystemNamespace
	if policy.Spec.ResourceClaimTemplate.Namespace != "" {
		namespace = policy.Spec.ResourceClaimTemplate.Namespace
	}

	// Prepare labels and annotations
	labels := map[string]string{
		"quota.miloapis.com/auto-created":        "true",
		"quota.miloapis.com/policy":              policy.Name,
		"quota.miloapis.com/gvk":                 fmt.Sprintf("%s.%s.%s", evalContext.GVK.Group, evalContext.GVK.Version, evalContext.GVK.Kind),
		"quota.miloapis.com/immediate-ownership": "true", // Flag to indicate immediate ownership
	}

	// Add template labels
	for key, value := range policy.Spec.ResourceClaimTemplate.Labels {
		labels[key] = value
	}

	annotations := map[string]string{
		"quota.miloapis.com/created-by":         "claim-creation-plugin",
		"quota.miloapis.com/created-at":         time.Now().Format(time.RFC3339),
		"quota.miloapis.com/resource-name":      evalContext.Object.GetName(),
		"quota.miloapis.com/policy":             policy.Name,
		"quota.miloapis.com/ownership-strategy": "immediate",
	}

	// Add template annotations
	for key, value := range policy.Spec.ResourceClaimTemplate.Annotations {
		annotations[key] = value
	}

	// Populate the ResourceRef with the unversioned reference to the resource being created
	spec.ResourceRef = quotav1alpha1.UnversionedObjectReference{
		APIGroup:  evalContext.GVK.Group,
		Kind:      evalContext.GVK.Kind,
		Name:      evalContext.Object.GetName(),
		Namespace: evalContext.Object.GetNamespace(),
	}

	// IMMEDIATE OWNERSHIP: Create owner reference from the admission context
	// The resource will exist by the time this claim is persisted since admission runs
	// during the resource creation process
	ownerReferences := []metav1.OwnerReference{}

	// Extract the resource metadata from the admission context
	if evalContext.Object != nil {
		ownerRef := metav1.OwnerReference{
			APIVersion:         evalContext.GVK.GroupVersion().String(),
			Kind:               evalContext.GVK.Kind,
			Name:               evalContext.Object.GetName(),
			UID:                evalContext.Object.GetUID(),
			Controller:         ptr.To(false), // Not a controller reference
			BlockOwnerDeletion: ptr.To(true),  // Block deletion until claim is cleaned up
		}

		// Only set owner reference if UID is available (resource has been persisted)
		if evalContext.Object.GetUID() != "" {
			ownerReferences = append(ownerReferences, ownerRef)
			p.logger.V(1).Info("Setting immediate owner reference on ResourceClaim",
				"targetUID", evalContext.Object.GetUID(),
				"targetName", evalContext.Object.GetName(),
				"targetKind", evalContext.GVK.Kind)
		} else {
			// UID not available yet - fall back to async ownership
			p.logger.V(1).Info("Target resource UID not available, will use async ownership",
				"targetName", evalContext.Object.GetName(),
				"targetKind", evalContext.GVK.Kind)
			annotations["quota.miloapis.com/ownership-strategy"] = "async-fallback"
		}
	}

	// Create the ResourceClaim
	claim := &quotav1alpha1.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    claimNamePrefix,
			Namespace:       namespace,
			Labels:          labels,
			Annotations:     annotations,
			OwnerReferences: ownerReferences, // Set immediately if possible
		},
		Spec: *spec,
	}

	return p.createResourceClaimObject(ctx, claim)
}

// createResourceClaimObject handles the actual creation of the ResourceClaim object
func (p *ClaimCreationPlugin) createResourceClaimObject(ctx context.Context, claim *quotav1alpha1.ResourceClaim) (string, string, error) {
	// Convert to unstructured and create
	claimBytes, err := json.Marshal(claim)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal ResourceClaim: %w", err)
	}

	var unstructuredClaim map[string]interface{}
	if err := json.Unmarshal(claimBytes, &unstructuredClaim); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal ResourceClaim to unstructured: %w", err)
	}

	unstructuredObj := &unstructured.Unstructured{Object: unstructuredClaim}
	unstructuredObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "quota.miloapis.com",
		Version: "v1alpha1",
		Kind:    "ResourceClaim",
	})

	gvr := schema.GroupVersionResource{
		Group:    "quota.miloapis.com",
		Version:  "v1alpha1",
		Resource: "resourceclaims",
	}

	// Create the ResourceClaim using dynamic client
	createdClaim, err := p.dynamicClient.Resource(gvr).Namespace(claim.Namespace).Create(ctx, unstructuredObj, metav1.CreateOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to create ResourceClaim: %w", err)
	}

	claimName := createdClaim.GetName()

	ownershipStrategy := "immediate"
	if len(claim.OwnerReferences) == 0 {
		ownershipStrategy = "async-fallback"
	}

	p.logger.Info("ResourceClaim created successfully",
		"claimName", claimName,
		"namespace", claim.Namespace,
		"ownershipStrategy", ownershipStrategy,
		"hasOwnerReferences", len(claim.OwnerReferences) > 0,
		"requestCount", len(claim.Spec.Requests))

	return claimName, claim.Namespace, nil
}
