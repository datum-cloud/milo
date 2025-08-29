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

// SetupQuotaWebhooksWithManager sets up a single dynamic webhook that can handle
// any GroupVersionKind configured in the MutatingWebhookConfiguration.
//
// This approach bypasses controller-runtime's automatic conversion webhook registration
// which causes issues when dealing with unregistered/external resource types (e.g. HTTPProxy).
func SetupQuotaWebhooksWithManager(mgr ctrl.Manager) error {
	resourceclaimlog.Info("Setting up dynamic quota webhook for automatic ResourceClaim creation")

	// Validate manager
	if mgr == nil {
		return fmt.Errorf("manager cannot be nil")
	}

	// Get supported resource types
	supportedGVKs := getSupportedResourceTypes()
	if len(supportedGVKs) == 0 {
		return fmt.Errorf("no supported resource types configured for ResourceClaim creation webhook")
	}

	// Get client from manager
	client := mgr.GetClient()
	if client == nil {
		return fmt.Errorf("failed to get client from manager")
	}

	// Create a single webhook handler that can process any configured GVK
	resourceclaimCreator := &DynamicResourceClaimCreator{
		Client:        client,
		SupportedGVKs: supportedGVKs,
	}

	// Get scheme from manager
	scheme := mgr.GetScheme()
	if scheme == nil {
		return fmt.Errorf("failed to get scheme from manager")
	}

	// Create the admission webhook directly using controller-runtime's admission package
	webhook := admission.WithCustomDefaulter(scheme, &unstructured.Unstructured{}, resourceclaimCreator)
	if webhook == nil {
		return fmt.Errorf("failed to create admission webhook")
	}

	// Get webhook server from manager
	webhookServer := mgr.GetWebhookServer()
	if webhookServer == nil {
		return fmt.Errorf("failed to get webhook server from manager")
	}

	// Register directly with the webhook server to avoid conversion check issues
	webhookPath := "/create-resourceclaim-miloapis-com-v1alpha1"
	webhookServer.Register(webhookPath, webhook)

	resourceclaimlog.Info("Successfully registered dynamic quota webhook",
		"path", webhookPath,
		"supportedGVKs", len(supportedGVKs),
		"resources", supportedGVKs)
	return nil
}

// +kubebuilder:webhook:path=/create-resourceclaim-miloapis-com-v1alpha1,mutating=true,failurePolicy=fail,sideEffects=NoneOnDryRun,groups=resourcemanager.miloapis.com;networking.datumapis.com,resources=projects;httpproxies,verbs=create,versions=v1alpha1;v1alpha,name=mresourceclaimcreation.quota.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system
// Note: This is a dynamic webhook that handles multiple resource types defined in getSupportedResourceTypes().
// The actual resources monitored are configured in config/webhook/manifests.yaml under quota.miloapis.com MutatingWebhookConfiguration.

// DynamicResourceClaimCreator handles automatic creation of ResourceClaims for any
// configured resource type defined in getSupportedResourceTypes().
//
// This implementation uses the admission request context to determine the GVK,
// avoiding the need for scheme registration of external resource types.
type DynamicResourceClaimCreator struct {
	Client        client.Client
	SupportedGVKs []schema.GroupVersionKind
}

// Default implements the webhook logic - creates ResourceClaims but doesn't mutate the incoming object.
// This implementation uses the admission request GVK directly, making it compatible with any
// configured resource type without requiring scheme registration.
func (r *DynamicResourceClaimCreator) Default(ctx context.Context, obj runtime.Object) error {
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get request from context: %w", err)
	}

	// Use the admission request GVK directly instead of trying to infer from object
	requestGVK := schema.GroupVersionKind{
		Group:   req.Kind.Group,
		Version: req.Kind.Version,
		Kind:    req.Kind.Kind,
	}

	resourceclaimlog.Info("Processing dynamic webhook request",
		"gvk", requestGVK,
		"name", req.Name,
		"namespace", req.Namespace)

	// Check if this GVK is in the supported list
	if !r.isSupportedGVK(requestGVK) {
		resourceclaimlog.Info("Skipping unsupported GVK", "gvk", requestGVK)
		return nil
	}

	// Don't create ResourceClaims for dry run requests
	if req.DryRun != nil && *req.DryRun {
		resourceclaimlog.Info("Skipping ResourceClaim creation for dry run request",
			"name", req.Name, "namespace", req.Namespace, "gvk", requestGVK)
		return nil
	}

	// Generate resource type using convention: {group}/{kind}
	resourceType := fmt.Sprintf("%s/%s", req.Kind.Group, req.Kind.Kind)

	resourceclaimlog.Info("Creating ResourceClaim for supported resource type",
		"resourceType", resourceType, "name", req.Name)

	// Create the ResourceClaim
	if err := r.createResourceClaim(ctx, req.Name, req.Kind.Kind, resourceType); err != nil {
		// Log the error but don't fail the original resource creation
		resourceclaimlog.Error(err, "Failed to create ResourceClaim",
			"resourceName", req.Name, "resourceType", resourceType, "gvk", requestGVK)
		// TODO: decide if this should fail which would block original resource creation
		// or just log the error and continue
		// return fmt.Errorf("failed to create ResourceClaim: %w", err)
	}

	return nil
}

// isSupportedGVK checks if the given GVK is in the list of supported resource types
func (r *DynamicResourceClaimCreator) isSupportedGVK(gvk schema.GroupVersionKind) bool {
	for _, supported := range r.SupportedGVKs {
		if supported.Group == gvk.Group &&
			supported.Version == gvk.Version &&
			supported.Kind == gvk.Kind {
			return true
		}
	}
	return false
}

// createResourceClaim creates a ResourceClaim for the given resource
func (r *DynamicResourceClaimCreator) createResourceClaim(ctx context.Context, resourceName, kind, resourceType string) error {
	// Generate a unique name for the ResourceClaim
	claimName := r.generateResourceClaimName(resourceName)

	// Check if ResourceClaim already exists
	var existingClaim quotav1alpha1.ResourceClaim
	err := r.Client.Get(ctx, types.NamespacedName{Name: claimName, Namespace: quotav1alpha1.MiloSystemNamespace}, &existingClaim)
	if err == nil {
		resourceclaimlog.Info("ResourceClaim already exists", "name", claimName, "namespace", quotav1alpha1.MiloSystemNamespace)
		return nil
	}

	// Create the ResourceClaim
	claim := &quotav1alpha1.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      claimName,
			Namespace: quotav1alpha1.MiloSystemNamespace,
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
	// Create the ResourceClaim
	if err := r.Client.Create(ctx, claim); err != nil {
		return fmt.Errorf("failed to create ResourceClaim: %w", err)
	}

	resourceclaimlog.Info("ResourceClaim created successfully",
		"claimName", claimName,
		"namespace", quotav1alpha1.MiloSystemNamespace,
		"resourceType", resourceType,
		"ownerKind", kind,
		"ownerName", resourceName)

	return nil
}

// generateResourceClaimName creates a unique name for ResourceClaim using a
// random alphanumeric suffix.
func (r *DynamicResourceClaimCreator) generateResourceClaimName(resourceName string) string {
	// Generate a random alphanumeric suffix to guarantee uniqueness
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	const suffixLength = 6

	suffix := make([]byte, suffixLength)
	for i := range suffix {
		suffix[i] = charset[rand.Intn(len(charset))]
	}

	// Create a readable name with owner and random suffix
	// Format: {resourceName}-claim-{random}
	return fmt.Sprintf("%s-claim-%s", resourceName, string(suffix))
}
