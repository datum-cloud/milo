package v1alpha1

import (
	"context"
	"fmt"
	"math/rand"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// log is for logging in this package.
var resourceclaimlog = logf.Log.WithName("resourceclaim-resource")

// getSupportedResourceTypes returns the list of resource types that should trigger ResourceClaim creation
//
// To add new resource types:
// 1. Add the resource type to the slice
// 2. Add corresponding webhook rules to config/webhook/manifests.yaml under the
// mresourceclaimcreation.quota.miloapis.com webhook configuration.
func getSupportedResourceTypes() []schema.GroupVersionKind {
	return []schema.GroupVersionKind{
		{
			Group:   "resourcemanager.miloapis.com",
			Version: "v1alpha1",
			Kind:    "Project",
		},
		{
			Group:   "networking.datumapis.com",
			Version: "v1alpha",
			Kind:    "HTTPProxy",
		},
	}
}

// SetupQuotaWebhooksWithManager is a generic function that automatically sets
// up a webhook with the manager for each resource type that should trigger
// ResourceClaim creation.
//
// By using getSupportedResourceTypes() to return a slice of resource types,
// no code changes are needed for each new resource type added in the future.
func SetupQuotaWebhooksWithManager(mgr ctrl.Manager) error {
	resourceclaimlog.Info("Setting up quota webhooks for automatic ResourceClaim creation")

	supportedTypes := getSupportedResourceTypes()
	resourceclaimCreator := &ResourceClaimCreator{Client: mgr.GetClient()}

	for _, resourceType := range supportedTypes {
		// Create unstructured object for this resource type
		gvk := schema.GroupVersionKind{
			Group:   resourceType.Group,
			Version: resourceType.Version,
			Kind:    resourceType.Kind,
		}

		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)

		// Set up webhook for this resource type
		ctrl.NewWebhookManagedBy(mgr).
			For(obj).
			WithDefaulter(resourceclaimCreator).
			Complete()

		resourceclaimlog.Info("Registered webhook for resource type",
			"group", resourceType.Group,
			"version", resourceType.Version,
			"kind", resourceType.Kind)
	}

	resourceclaimlog.Info("Successfully set up quota webhooks", "resourceTypeCount", len(supportedTypes))
	return nil
}

// +kubebuilder:webhook:path=/mutate-create-resourceclaim-miloapis-com-v1alpha1,mutating=true,failurePolicy=fail,sideEffects=NoneOnDryRun,groups=networking.datumapis.com,resources=httpproxies,verbs=create,versions=v1alpha1,name=mresourceclaimcreation.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system
// +kubebuilder:webhook:path=/mutate-create-resourceclaim-miloapis-com-v1alpha1,mutating=true,failurePolicy=fail,sideEffects=NoneOnDryRun,groups=resourcemanager.miloapis.com,resources=projects,verbs=create,versions=v1alpha1,name=mresourceclaimcreation.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

// ResourceClaimCreator handles automatic creation of ResourceClaims for any
// configured resource type defined in getSupportedResourceTypes().
type ResourceClaimCreator struct {
	Client client.Client
}

// Default implements the webhook logic - creates ResourceClaims but doesn't mutate the incoming object
// This webhook is generic and works with any resource type by using the {group}/{kind} convention.
func (r *ResourceClaimCreator) Default(ctx context.Context, obj runtime.Object) error {
	resourceclaimlog.Info("Executing ResourceClaim creation webhook", "obj", obj)
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get request from context: %w", err)
	}

	// Don't create ResourceClaims for dry run requests
	if req.DryRun != nil && *req.DryRun {
		resourceclaimlog.Info("Skipping ResourceClaim creation for dry run request", "name", req.Name, "namespace", req.Namespace)
		return nil
	}

	// Extract basic information from the request
	kind := req.Kind.Kind
	group := req.Kind.Group
	name := req.Name
	namespace := quotav1alpha1.MiloSystemNamespace

	resourceclaimlog.Info("Processing resource for ResourceClaim creation",
		"name", name,
		"namespace", namespace,
		"kind", kind,
		"group", group)

	// Generate resource type using convention: {group}/{kind}
	// This makes the webhook generic and automatically handles any resource
	// type without the need to hardcode each one.
	resourceType := fmt.Sprintf("%s/%s", group, kind)

	resourceclaimlog.Info("Generated resource type", "resourceType", resourceType)

	// Create the ResourceClaim
	if err := r.createResourceClaim(ctx, name, namespace, kind, resourceType); err != nil {
		// Log the error but don't fail the original resource creation
		resourceclaimlog.Error(err, "Failed to create ResourceClaim", "resourceName", name, "resourceType", resourceType)
		// TODO: decide if this should fail which would block original resource creation
		// or just log the error and continue
		// return fmt.Errorf("failed to create ResourceClaim: %w", err)
	}

	return nil
}

// createResourceClaim creates a ResourceClaim for the given resource
func (r *ResourceClaimCreator) createResourceClaim(ctx context.Context, resourceName, namespace, kind, resourceType string) error {
	// Generate a unique name for the ResourceClaim
	claimName := r.generateResourceClaimName(resourceName)

	// Check if ResourceClaim already exists
	var existingClaim quotav1alpha1.ResourceClaim
	err := r.Client.Get(ctx, types.NamespacedName{Name: claimName, Namespace: namespace}, &existingClaim)
	if err == nil {
		resourceclaimlog.Info("ResourceClaim already exists", "name", claimName, "namespace", namespace)
		return nil
	}

	// Create the ResourceClaim
	claim := &quotav1alpha1.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      claimName,
			Namespace: namespace,
			Labels: map[string]string{
				"quota.miloapis.com/auto-created": "true",
			},
			Annotations: map[string]string{
				"quota.miloapis.com/created-by": "resourceclaim-creator-webhook",
			},
		},
		Spec: quotav1alpha1.ResourceClaimSpec{
			OwnerInstanceRef: quotav1alpha1.OwnerInstanceRef{
				Kind: kind,
				Name: resourceName,
			},
			Requests: []quotav1alpha1.ResourceRequest{
				{
					ResourceType: resourceType,
					Amount:       1,
					Dimensions:   map[string]string{},
				},
			},
		},
	}
	resourceclaimlog.Info("Creating ResourceClaim", "claimName", claimName, "namespace", namespace, "resourceType", resourceType)

	// Create the ResourceClaim
	if err := r.Client.Create(ctx, claim); err != nil {
		return fmt.Errorf("failed to create ResourceClaim: %w", err)
	}

	resourceclaimlog.Info("ResourceClaim created successfully",
		"claimName", claimName,
		"namespace", namespace,
		"resourceType", resourceType,
		"ownerKind", kind,
		"ownerName", resourceName)

	return nil
}

// generateResourceClaimName creates a unique name for ResourceClaim using a
// random alphanumeric suffix.
func (r *ResourceClaimCreator) generateResourceClaimName(resourceName string) string {
	// Generate a random alphanumeric suffix to guarantee uniqueness
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	const suffixLength = 6

	suffix := make([]byte, suffixLength)
	for i := range suffix {
		suffix[i] = charset[rand.Intn(len(charset))]
	}

	// Create a readable name with owner and random suffix
	// Format: {resourceName}-{kind-lower}-claim-{random}
	return fmt.Sprintf("%s-%s-claim-%s", resourceName, "resource", string(suffix))
}
