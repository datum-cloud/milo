package v1alpha1

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	"go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
)

// log is for logging in this package.
var organizationlog = logf.Log.WithName("organization-resource")

// +kubebuilder:webhook:path=/validate-resourcemanager-miloapis-com-v1alpha1-organization,mutating=false,failurePolicy=fail,sideEffects=None,groups=resourcemanager.miloapis.com,resources=organizations,verbs=create,versions=v1alpha1,name=vorganization.datum.net,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

// SetupWebhooksWithManager sets up all resourcemanager.miloapis.com webhooks
func SetupOrganizationWebhooksWithManager(mgr ctrl.Manager, systemNamespace string, organizationOwnerRoleName string) error {
	organizationlog.Info("Setting up resourcemanager.miloapis.com organization webhooks")

	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.Organization{}).
		WithValidator(&OrganizationValidator{
			client:          mgr.GetClient(),
			systemNamespace: systemNamespace,
			ownerRoleName:   organizationOwnerRoleName,
		}).
		Complete()
}

// OrganizationValidator validates Organizations
type OrganizationValidator struct {
	client          client.Client
	decoder         admission.Decoder
	systemNamespace string
	ownerRoleName   string
}

func (v *OrganizationValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	org := obj.(*v1alpha1.Organization)
	organizationlog.Info("Validating Organization", "name", org.Name)

	// Create namespace and PolicyBinding on Organization Create operation
	if err := v.createOrganizationNamespace(ctx, org); err != nil {
		return nil, fmt.Errorf("failed to create organization namespace: %w", err)
	}

	if err := v.createOwnerPolicyBinding(ctx, org); err != nil {
		return nil, fmt.Errorf("failed to create owner policy binding: %w", err)
	}

	return nil, nil
}

func (v *OrganizationValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *OrganizationValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// lookupUser retrieves the User resource from the iam.miloapis.com API
func (v *OrganizationValidator) lookupUser(ctx context.Context, username string) (*iamv1alpha1.User, error) {
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get request from context: %w", err)
	}

	// TODO: Determine if we can actually use the UID from the User object in
	//       the UserInfo of the request. Likely need to configure the OIDC
	//       authorization to map the UID from the JWT claims.
	foundUser := &iamv1alpha1.User{}
	if err := v.client.Get(ctx, client.ObjectKey{Name: req.UserInfo.Username}, foundUser); err != nil {
		return nil, fmt.Errorf("failed to get user '%s' from iam.miloapis.com API: %w", username, err)
	}

	return foundUser, nil
}

// createOwnerPolicyBinding creates a PolicyBinding for the organization owner
func (v *OrganizationValidator) createOwnerPolicyBinding(ctx context.Context, org *v1alpha1.Organization) error {
	organizationlog.Info("Attempting to create PolicyBinding for new organization", "organization", org.Name)

	// Look up the user in the iam API
	user, err := v.lookupUser(ctx, org.Name)
	if err != nil {
		return fmt.Errorf("failed to lookup user: %w", err)
	}

	// Build the PolicyBinding
	policyBinding := &iamv1alpha1.PolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-owner", org.Name),
			Namespace: fmt.Sprintf("organization-%s", org.Name),
		},
		Spec: iamv1alpha1.PolicyBindingSpec{
			RoleRef: iamv1alpha1.RoleReference{
				Name:      v.ownerRoleName,
				Namespace: v.systemNamespace,
			},
			Subjects: []iamv1alpha1.Subject{
				{
					Kind: "User",
					Name: user.Name,
					UID:  string(user.GetUID()),
				},
			},
			TargetRef: iamv1alpha1.TargetReference{
				APIGroup: v1alpha1.GroupVersion.Group,
				Kind:     "Organization",
				Name:     org.Name,
				UID:      string(org.UID),
			},
		},
	}

	if err := v.client.Create(ctx, policyBinding); err != nil {
		return fmt.Errorf("failed to create policy binding resource: %w", err)
	}

	return nil
}

// createOrganizationNamespace creates a namespace for organization-scoped resources
func (v *OrganizationValidator) createOrganizationNamespace(ctx context.Context, org *v1alpha1.Organization) error {
	namespaceName := fmt.Sprintf("organization-%s", org.Name)
	organizationlog.Info("Creating namespace for organization", "organization", org.Name, "namespace", namespaceName)

	// Build the namespace object
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
			Labels: map[string]string{
				"resourcemanager.miloapis.com/organization": org.Name,
				"resourcemanager.miloapis.com/type":         "organization",
			},
		},
	}

	if err := v.client.Create(ctx, namespace); err != nil {
		return fmt.Errorf("failed to create namespace resource: %w", err)
	}

	return nil
}
