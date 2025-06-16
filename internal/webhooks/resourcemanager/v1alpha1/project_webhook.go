package v1alpha1

import (
	"context"
	"fmt"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	"go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var projectlog = logf.Log.WithName("project-resource")

// SetupWebhooksWithManager sets up all resourcemanager.miloapis.com webhooks
func SetupProjectWebhooksWithManager(mgr ctrl.Manager, systemNamespace string, projectOwnerRoleName string) error {
	projectlog.Info("Setting up resourcemanager.miloapis.com project webhooks")

	ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.Project{}).
		WithValidator(&ProjectValidator{
			Client:               mgr.GetClient(),
			SystemNamespace:      systemNamespace,
			ProjectOwnerRoleName: projectOwnerRoleName,
		}).
		WithDefaulter(&ProjectMutator{
			client: mgr.GetClient(),
		}).
		Complete()
	return nil
}

// +kubebuilder:webhook:path=/mutate-resourcemanager-miloapis-com-v1alpha1-project,mutating=true,failurePolicy=fail,sideEffects=None,groups=resourcemanager.miloapis.com,resources=projects,verbs=create,versions=v1alpha1,name=mproject.datum.net,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

// ProjectMutator mutates Projects to add owner references and the owning
// organization based on the request context.
type ProjectMutator struct {
	client client.Client
}

func (m *ProjectMutator) Default(ctx context.Context, obj runtime.Object) error {
	project, ok := obj.(*v1alpha1.Project)
	if !ok {
		return fmt.Errorf("failed to cast object to Project")
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get request from context: %w", err)
	}

	requestContextOrgID, ok := req.UserInfo.Extra[v1alpha1.OrganizationNameLabel]
	if !ok {
		errMsg := fmt.Sprintf("request context does not have the required organization name label '%s'", v1alpha1.OrganizationNameLabel)
		projectlog.Error(fmt.Errorf(errMsg), errMsg)
		return fmt.Errorf(errMsg)
	}

	org := &v1alpha1.Organization{}
	if err := m.client.Get(ctx, client.ObjectKey{Name: requestContextOrgID[0]}, org); err != nil {
		return fmt.Errorf("failed to get organization '%s': %w", requestContextOrgID[0], err)
	}

	// If the request context has had an org id injected, default the parent to
	// the org. Once we introduce folders, this will need to change to leave the
	// value alone, and allow validation to ensure it's a valid parent folder.
	project.Spec.OwnerRef = v1alpha1.OwnerReference{
		Kind: "Organization",
		Name: org.Name,
	}

	project.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       "Organization",
			Name:       org.Name,
			UID:        org.GetUID(),
		},
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-resourcemanager-miloapis-com-v1alpha1-project,mutating=false,failurePolicy=fail,sideEffects=None,groups=resourcemanager.miloapis.com,resources=projects,verbs=create,versions=v1alpha1,name=vproject.datum.net,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

// ProjectValidator validates Projects and creates associated PolicyBindings for owners.
type ProjectValidator struct {
	Client               client.Client
	decoder              admission.Decoder
	SystemNamespace      string
	ProjectOwnerRoleName string
}

// ValidateCreate validates the Project and creates the associated PolicyBinding
// to provide the authenticated user with ownership access to the project.
func (v *ProjectValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	project, ok := obj.(*v1alpha1.Project)
	if !ok {
		return nil, fmt.Errorf("failed to cast object to Project")
	}

	projectlog.Info("Validating Project", "name", project.Name)
	errs := field.ErrorList{}

	if project.Spec.OwnerRef.Kind == "" {
		errs = append(errs, field.Invalid(field.NewPath("spec.ownerRef.kind"), project.Spec.OwnerRef.Kind, "must be set"))
	}

	if project.Spec.OwnerRef.Kind != "Organization" {
		errs = append(errs, field.Invalid(field.NewPath("spec.ownerRef.kind"), project.Spec.OwnerRef.Kind, "must be 'Organization'"))
	}

	if project.Spec.OwnerRef.Name == "" {
		errs = append(errs, field.Invalid(field.NewPath("spec.ownerRef.name"), project.Spec.OwnerRef.Name, "must be set"))
	}

	if len(errs) > 0 {
		return nil, errors.NewInvalid(project.GroupVersionKind().GroupKind(), project.Name, errs)
	}

	if err := v.createOwnerPolicyBinding(ctx, project); err != nil {
		return nil, fmt.Errorf("failed to create owner policy binding: %w", err)
	}

	return nil, nil
}

func (v *ProjectValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *ProjectValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// lookupUser retrieves the User resource from the iam.miloapis.com API
func (v *ProjectValidator) lookupUser(ctx context.Context, username string) (*iamv1alpha1.User, error) {
	// TODO: Determine if we can actually use the UID from the User object in
	//       the UserInfo of the request. Likely need to configure the OIDC
	//       authorization to map the UID from the JWT claims.
	foundUser := &iamv1alpha1.User{}
	if err := v.Client.Get(ctx, client.ObjectKey{Name: username}, foundUser); err != nil {
		return nil, fmt.Errorf("failed to get user '%s' from iam.miloapis.com API: %w", username, err)
	}

	return foundUser, nil
}

// createOwnerPolicyBinding creates a PolicyBinding for the project owner
func (v *ProjectValidator) createOwnerPolicyBinding(ctx context.Context, project *v1alpha1.Project) error {
	projectlog.Info("Attempting to create PolicyBinding for new project", "project", project.Name)
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get request from context: %w", err)
	}

	// Look up the user in the iam API
	foundUser, err := v.lookupUser(ctx, req.UserInfo.Username)
	if err != nil {
		return fmt.Errorf("failed to lookup user: %w", err)
	}

	// Build the PolicyBinding
	policyBinding := &iamv1alpha1.PolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-owner", project.Name),
			// Create the policy binding in the organization's namespace that the
			// project belongs to.
			//
			// TODO: Will need to re-consider this when the folder type can be
			//       introduced as a parent. Maybe we have an Owner field in the spec?
			Namespace: fmt.Sprintf("organization-%s", project.Spec.OwnerRef.Name),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v1alpha1.GroupVersion.String(),
					Kind:       "Project",
					Name:       project.Name,
					UID:        project.UID,
				},
			},
		},
		Spec: iamv1alpha1.PolicyBindingSpec{
			RoleRef: iamv1alpha1.RoleReference{
				Name:      v.ProjectOwnerRoleName,
				Namespace: v.SystemNamespace,
			},
			Subjects: []iamv1alpha1.Subject{
				{
					Kind: "User",
					Name: req.UserInfo.Username,
					UID:  string(foundUser.GetUID()),
				},
			},
			TargetRef: iamv1alpha1.TargetReference{
				APIGroup: v1alpha1.GroupVersion.Group,
				Kind:     "Project",
				Name:     project.Name,
				UID:      string(project.UID),
			},
		},
	}

	// Create the PolicyBinding resource
	if err := v.Client.Create(ctx, policyBinding); err != nil {
		return fmt.Errorf("failed to create policy binding resource: %w", err)
	}

	return nil
}
