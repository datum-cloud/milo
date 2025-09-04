package quota

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
)

const (
	// DefaultResourceGrantSuffix is appended to resource names when creating default ResourceGrant objects.
	// This helps distinguish automatically created grants from manually created ones and provides
	// a consistent naming pattern for easier identification and management,
	// however it is *not* used in the logic that finds the specific default
	// grant to determine if it already exists.
	DefaultResourceGrantSuffix = "-default-grant"

	// ProjectResourceType is the qualified resource name for the Project
	// resource type, which is used to create a default ResourceGrant for the maximum number of Project
	// instances for an Organization.
	ProjectResourceType = "resourcemanager.miloapis.com/Project"

	// HTTPProxyResourceType is the fully qualified resource name for the
	// HTTPProxy resourc typee, which is used to create a default ResourceGrant for the maximum number of HTTPProxy
	// instances for a Project.
	HTTPProxyResourceType = "networking.datumapis.com/HTTPProxy"
)

// The DefaultResourceGrantController watches both Project and Organization
// resources and automatically creates default ResourceGrant objects when
// either resource is created, removing the need for manual configuration. It
// uses the IsDefault and ResourceType fields on the ResourceGrantSpec
// to find the specific default grant and determine if it has already been created for the Project or
// Organization; ensuring that the default grant is created only once.
// TODO: Move the responsibility of default grant creation to Kyverno policies.
type DefaultResourceGrantController struct {
	// Client provides access to the Kubernetes API for CRUD operations
	Client client.Client
	// Scheme is used for setting up owner references between resources
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceregistrations,verbs=get;list;watch
// +kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=organizations,verbs=get;list;watch
// +kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=projects,verbs=get;list;watch

// The Reconcile function reconciles Project and Organization resources when
// they are created or changed in the APIServer and is responsible for creating
// default ResourceGrants for both resources on creation.
//
// Responsibilites:
// 1. Determines whether a Project or Organization triggered the reconciliation.
// 2. Delegates default ResourceGrant creation to the appropriate function.
func (r *DefaultResourceGrantController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Organization or Project for default ResourceGrant creation", "NAMESPACED_NAME", req.NamespacedName, "NAME", req.Name)
	// Attempts to fetch an Organization by its name, as it is a cluster-scoped resource.
	org := &resourcemanagerv1alpha1.Organization{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: req.Name}, org); err == nil {
		// If the Organization was found, proceed to create default grant(s) for it
		return r.createDefaultGrantsForOrganization(ctx, org)
	} else if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("failed to get Organization: %w", err)
	}

	// If the Organization is not found in the above logic, attempts to fetch a
	// Project by its name.
	project := &resourcemanagerv1alpha1.Project{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: req.Name}, project); err == nil {
		// If the Project was found, proceed to create default grant(s) for it
		return r.createDefaultGrantsForProject(ctx, project)
	} else if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("failed to get Project: %w", err)
	}

	// Neither Organization or Project were found
	logger.Info("Resource not found", "namespacedName", req.NamespacedName)
	return ctrl.Result{}, nil
}

// createDefaultGrantsForOrganization handles the creation of default
// ResourceGrants on the Organizational level when the Organization is initially
// created.
//
// The following default grants created on Organization creation:
// - Max Projects per Organization
//
// Responsibilities:
//  1. Validates that the required ResourceRegistrations exists and is active before creating grants
//  2. Determines if the specific default ResourceGrant already exists using the
//     IsDefault field and resource type name.
//  3. Creates a new ResourceGrant if necessary:
//     - Configures an allowance that specifies the quota limit for the resource type
//     - Uses the Organization name as the namespace for the grant
//     - Sets IsDefault = true to distinguish from manually created grants
//     - Creates an empty DimensionSelector to match all resources until dimensional constrains are added in a future enhancement.
func (r *DefaultResourceGrantController) createDefaultGrantsForOrganization(ctx context.Context, org *resourcemanagerv1alpha1.Organization) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Organization for default ResourceGrant creation", "organization", org)

	// TODO: add finalizer
	if !org.DeletionTimestamp.IsZero() {
		logger.Info("Organization is being deleted, skipping default grant creation", "organization", org)
		return ctrl.Result{}, nil
	}

	// Validates that the required ResourceRegistration exists before creating the grant
	if err := ValidateResourceRegistrations(ctx, r.Client, []string{ProjectResourceType}); err != nil {
		logger.Info("Cannot create default Project ResourceGrant - required ResourceRegistration not found",
			"org", org.Name, "resourceType", ProjectResourceType, "error", err)
		// Returns success to avoid retrying
		return ctrl.Result{}, nil
	}

	// Lists existing ResourceGrants for this Organization to determine if a
	// default grant for the specific resource type has already been created.
	existingGrants := &quotav1alpha1.ResourceGrantList{}
	err := r.Client.List(ctx, existingGrants, client.InNamespace(org.Name))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list existing ResourceGrants: %w", err)
	}

	// Iterates the list of grants and the allowances within each and uses the
	// resource type name to determine if the specific default grant has already
	// been created.
	for _, grant := range existingGrants.Items {
		if grant.Spec.IsDefault {
			// Checks if this default grant covers Project resources
			for _, allowance := range grant.Spec.Allowances {
				if allowance.ResourceType == ProjectResourceType {
					logger.Info("Default Project ResourceGrant already exists", "org", org.Name, "grant", grant.Name)
					return ctrl.Result{}, nil
				}
			}
		}
	}

	// Constructs the default ResourceGrant for maximum Project instances for the Organization
	maxProjectGrantName := org.Name + "-max-projects" + DefaultResourceGrantSuffix
	grant := &quotav1alpha1.ResourceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      maxProjectGrantName,
			Namespace: quotav1alpha1.MiloSystemNamespace,
		},
		Spec: quotav1alpha1.ResourceGrantSpec{
			OwnerInstanceRef: quotav1alpha1.OwnerInstanceRef{
				Kind: "Organization",
				Name: org.Name,
			},
			IsDefault: true,
			Allowances: []quotav1alpha1.Allowance{
				{
					ResourceType: ProjectResourceType,
					Buckets: []quotav1alpha1.Bucket{
						{
							// Max 5 Projects per Organization
							Amount: 5,
							// Empty DimensionSelector to match all objects
							// until specific dimensions are implemented post MVP
							DimensionSelector: metav1.LabelSelector{},
						},
					},
				},
			},
		},
	}

	if err := r.Client.Create(ctx, grant); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create default ResourceGrant %s for Organization %s: %w", maxProjectGrantName, org.Name, err)
	}

	logger.Info("Created default Project ResourceGrant for Organization", "org", org.Name, "grant", maxProjectGrantName)
	return ctrl.Result{}, nil
}

// createDefaultGrantsForProject handles the creation of default
// ResourceGrants on the Project level when the Project is initially
// created.
//
// The following default grants created on Project creation:
// - Max HTTPProxies per Project
//
// Responsibilities:
//  1. Validates that the required ResourceRegistrations exists and is active before creating grants
//  2. Determines if the specific default ResourceGrant already exists using the
//     IsDefault field and resource type name.
//  3. Creates a new ResourceGrant if necessary:
//     - Configures an allowance that specifies the quota limit for the resource type
//     - Uses the Project name as the namespace for the grant
//     - Sets IsDefault = true to distinguish from manually created grants
//     - Creates an empty DimensionSelector to match all resources until dimensional constrains are added in a future enhancement.
func (r *DefaultResourceGrantController) createDefaultGrantsForProject(ctx context.Context, project *resourcemanagerv1alpha1.Project) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Project for default ResourceGrant creation", "project", project)

	if !project.DeletionTimestamp.IsZero() {
		logger.Info("Project is being deleted, skipping default grant creation", "name", project.Name)
		return ctrl.Result{}, nil
	}

	// Validate that the required ResourceRegistration exists before creating the grant
	if err := ValidateResourceRegistrations(ctx, r.Client, []string{HTTPProxyResourceType}); err != nil {
		logger.Info("Cannot create default HTTPProxy ResourceGrant - required registration not found",
			"project", project.Name, "resourceType", HTTPProxyResourceType, "error", err)
		// Return success to avoid retrying
		return ctrl.Result{}, nil
	}

	// List existing ResourceGrants for this Project to determine if a
	// default grant for the specific resource type has already been created
	existingGrants := &quotav1alpha1.ResourceGrantList{}
	err := r.Client.List(ctx, existingGrants, client.InNamespace("project-"+project.Name))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list existing ResourceGrants: %w", err)
	}

	// Check if there is already a default grant created for max HTTPProxy
	// resources for the Project
	for _, grant := range existingGrants.Items {
		if grant.Spec.IsDefault {
			// Check if this default grant covers HTTPProxy resources
			for _, allowance := range grant.Spec.Allowances {
				if allowance.ResourceType == HTTPProxyResourceType {
					logger.Info("Default HTTPProxy ResourceGrant already exists for Project", "project", project.Name, "grant", grant.Name)
					return ctrl.Result{}, nil
				}
			}
		}
	}

	// Construct the default ResourceGrant for max HTTPProxy instances for the Project
	maxHTTPProxyGrantName := project.Name + "-max-httpproxies" + DefaultResourceGrantSuffix
	defaultGrant := &quotav1alpha1.ResourceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      maxHTTPProxyGrantName,
			Namespace: quotav1alpha1.MiloSystemNamespace,
		},
		Spec: quotav1alpha1.ResourceGrantSpec{
			OwnerInstanceRef: quotav1alpha1.OwnerInstanceRef{
				Kind: "Project",
				Name: project.Name,
			},
			IsDefault: true,
			Allowances: []quotav1alpha1.Allowance{
				{
					ResourceType: HTTPProxyResourceType,
					Buckets: []quotav1alpha1.Bucket{
						{
							// Max 5 HTTPProxy instances per Project
							Amount: 5,
							// Empty DimensionSelector to match all objects
							// until specific dimensions are implemented post MVP
							DimensionSelector: metav1.LabelSelector{},
						},
					},
				},
			},
		},
	}

	// Create the default grant
	if err := r.Client.Create(ctx, defaultGrant); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create default ResourceGrant %s for Project %s: %w", maxHTTPProxyGrantName, project.Name, err)
	}

	logger.Info("Created default HTTPProxy ResourceGrant for Project", "project", project.Name, "grant", maxHTTPProxyGrantName, "resourceType", HTTPProxyResourceType)
	return ctrl.Result{}, nil
}

// SetupWithManager configures the DefaultResourceGrantController with the
// controller-runtime Manager to watch both Project and Organization resources.
// Uses GenerationChangedPredicate to only trigger on spec changes, avoiding reconciliations
// for metadata-only updates  or status changes.
func (r *DefaultResourceGrantController) SetupWithManager(mgr ctrl.Manager) error {
	// Only reconcile when generation changes (spec updates), not metadata/status updates
	specChangePredicate := predicate.GenerationChangedPredicate{}

	return ctrl.NewControllerManagedBy(mgr).
		Watches(
			&resourcemanagerv1alpha1.Organization{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				return []reconcile.Request{
					{NamespacedName: types.NamespacedName{
						Name: obj.GetName(),
					}},
				}
			}),
			builder.WithPredicates(specChangePredicate),
		).
		Watches(
			&resourcemanagerv1alpha1.Project{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				return []reconcile.Request{
					{NamespacedName: types.NamespacedName{
						Name: obj.GetName(),
					}},
				}
			}),
			builder.WithPredicates(specChangePredicate),
		).
		Named("default-resourcegrant").
		Complete(r)
}
