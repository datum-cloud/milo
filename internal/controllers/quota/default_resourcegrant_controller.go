package quota

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
)

const (
	DefaultResourceGrantSuffix = "-default-grant"

	// Fully qualified resource name for the Project resource, which is used
	// to create a default ResourceGrant for the maximum number of Project
	// instances for an Organization.
	ProjectResourceTypeName = "resourcemanager.miloapis.com/Project"

	// Fully qualified resource name for the HTTPProxy resource, which is used
	// to create a default ResourceGrant for the maximum number of HTTPProxy
	// instances for a Project.
	HTTPProxyResourceTypeName = "networking.datumapis.com/HTTPProxy"
)

type DefaultResourceGrantController struct {
	// Client provides access to the Kubernetes API for CRUD operations
	Client client.Client
	// Scheme is used for setting up owner references between resources
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceregistrations,verbs=get;list;watch

// Handles the creation of default ResourceGrants when Organization or Project resources
// are created. It watches both Organization and Project resources and contains
// a function for each to create the default grants depending on which resource triggered the reconciliation.
func (r *DefaultResourceGrantController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Attempt to fetch an Organization by its namespace (organization name)
	org := &resourcemanagerv1alpha1.Organization{}
	if err := r.Client.Get(ctx, req.NamespacedName, org); err == nil {
		// If the Organization was found, proceed to create default grants for it
		return r.createDefaultGrantsForOrganization(ctx, org)
	} else if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("failed to get Organization: %w", err)
	}

	// If the Organization is not found in the above logic, attempt to fetch a
	// Project by its namespace (project name).
	project := &resourcemanagerv1alpha1.Project{}
	if err := r.Client.Get(ctx, req.NamespacedName, project); err == nil {
		// If the Project was found, proceed to create default grants for it
		return r.createDefaultGrantsForProject(ctx, project)
	} else if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("failed to get Project: %w", err)
	}

	// Neither Organization or Project were found
	logger.Info("Resource not found", "namespacedName", req.NamespacedName)
	return ctrl.Result{}, nil
}

// Creates default ResourceGrants for newly created Organizations, which
// initially includes:
//
// - Max Project instances for the Organization
func (r *DefaultResourceGrantController) createDefaultGrantsForOrganization(ctx context.Context, org *resourcemanagerv1alpha1.Organization) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Organization for default ResourceGrant creation", "organizationName", org.Name)

	if !org.DeletionTimestamp.IsZero() {
		logger.Info("Organization is being deleted, skipping default grant creation", "name", org.Name)
		return ctrl.Result{}, nil
	}

	// Validate that the required ResourceRegistration exists before creating the grant
	if err := ValidateResourceRegistrations(ctx, r.Client, []string{ProjectResourceTypeName}); err != nil {
		logger.Info("Cannot create default Project ResourceGrant - required registration not found",
			"org", org.Name, "resourceType", ProjectResourceTypeName, "error", err)
		// Return success to avoid retrying
		return ctrl.Result{}, nil
	}

	// Check if the default ResourceGrant for max Projects already exists for this Organization
	maxProjectGrantName := org.Name + "-max-project" + DefaultResourceGrantSuffix
	existingGrant := &quotav1alpha1.ResourceGrant{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      maxProjectGrantName,
		Namespace: org.Name,
	}, existingGrant)
	if err == nil {
		// The default max Projects grant already exists, so don't create another one.
		logger.Info("Default Project ResourceGrant already exists", "org", org.Name, "grant", maxProjectGrantName)
		return ctrl.Result{}, nil
	}

	if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("failed to check for existing ResourceGrant: %w", err)
	}

	// Construct the default ResourceGrant for maximum Project instances for the Organization
	grant := &quotav1alpha1.ResourceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      maxProjectGrantName,
			Namespace: org.Name,
		},
		Spec: quotav1alpha1.ResourceGrantSpec{
			OwnerRef: quotav1alpha1.OwnerRef{
				APIGroup: "resourcemanager.miloapis.com",
				Kind:     "Organization",
				Name:     org.Name,
			},
			Allowances: []quotav1alpha1.Allowance{
				{
					ResourceTypeName: ProjectResourceTypeName,
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

	// Create the default grant
	if err := r.Client.Create(ctx, grant); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create default ResourceGrant %s for Organization %s: %w", maxProjectGrantName, org.Name, err)
	}

	logger.Info("Created default Project ResourceGrant for Organization", "org", org.Name, "grant", maxProjectGrantName)
	return ctrl.Result{}, nil
}

// Creates default ResourceGrants for newly created Projects, which
// initially includes:
//
// - Max HTTPProxy instances for the Project
func (r *DefaultResourceGrantController) createDefaultGrantsForProject(ctx context.Context, project *resourcemanagerv1alpha1.Project) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Project for default ResourceGrant creation", "projectName", project.Name)

	if !project.DeletionTimestamp.IsZero() {
		logger.Info("Project is being deleted, skipping default grant creation", "name", project.Name)
		return ctrl.Result{}, nil
	}

	// Validate that the required ResourceRegistration exists before creating the grant
	if err := ValidateResourceRegistrations(ctx, r.Client, []string{HTTPProxyResourceTypeName}); err != nil {
		logger.Info("Cannot create default HTTPProxy ResourceGrant - required registration not found",
			"project", project.Name, "resourceType", HTTPProxyResourceTypeName, "error", err)
		// Return success to avoid retrying
		return ctrl.Result{}, nil
	}

	// Check if the default grant already exists for the HTTPProxy resource type
	maxHTTPProxyGrantName := project.Name + "-max-httpproxy" + DefaultResourceGrantSuffix
	existingGrant := &quotav1alpha1.ResourceGrant{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      maxHTTPProxyGrantName,
		Namespace: project.Name,
	}, existingGrant)
	if err == nil {
		// Default HTTPProxy ResourceGrant already exists for this resource type, so don't create another one
		logger.Info("Default HTTPProxy ResourceGrant already exists for Project", "project", project.Name, "grant", maxHTTPProxyGrantName)
		return ctrl.Result{}, nil
	}

	if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("failed to check for existing ResourceGrant: %w", err)
	}

	// Construct the default ResourceGrant for max HTTPProxy instances for the Organization
	grant := &quotav1alpha1.ResourceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      maxHTTPProxyGrantName,
			Namespace: project.Name,
		},
		Spec: quotav1alpha1.ResourceGrantSpec{
			OwnerRef: quotav1alpha1.OwnerRef{
				APIGroup: "resourcemanager.miloapis.com",
				Kind:     "Project",
				Name:     project.Name,
			},
			Allowances: []quotav1alpha1.Allowance{
				{
					ResourceTypeName: HTTPProxyResourceTypeName,
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
	if err := r.Client.Create(ctx, grant); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create default ResourceGrant %s for Project %s: %w", maxHTTPProxyGrantName, project.Name, err)
	}

	logger.Info("Created default HTTPProxy ResourceGrant for Project", "project", project.Name, "grant", maxHTTPProxyGrantName, "resourceType", HTTPProxyResourceTypeName)
	return ctrl.Result{}, nil
}

// SetupWithManager configures the DefaultResourceGrantController with the
// controller-runtime Manager to watch both Project and Organization resources.
func (r *DefaultResourceGrantController) SetupWithManager(mgr ctrl.Manager) error {
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
		).
		Named("default-resourcegrant").
		Complete(r)
}
