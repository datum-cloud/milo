package v1alpha1

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
)

// log is for logging in this package.
var organizationlog = logf.Log.WithName("organization-resource")

// +kubebuilder:webhook:path=/validate-resourcemanager-miloapis-com-v1alpha1-organization,mutating=false,failurePolicy=fail,sideEffects=None,groups=resourcemanager.miloapis.com,resources=organizations,verbs=create,versions=v1alpha1,name=vorganization.datum.net,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

// SetupWebhooksWithManager sets up all resourcemanager.miloapis.com webhooks
func SetupOrganizationWebhooksWithManager(mgr ctrl.Manager, systemNamespace string, organizationOwnerRoleName string) error {
	organizationlog.Info("Setting up resourcemanager.miloapis.com organization webhooks")

	return ctrl.NewWebhookManagedBy(mgr).
		For(&resourcemanagerv1alpha1.Organization{}).
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
	org := obj.(*resourcemanagerv1alpha1.Organization)
	organizationlog.Info("Validating Organization", "name", org.Name)

	// Create namespace and PolicyBinding on Organization Create operation
	if err := v.createOrganizationNamespace(ctx, org); err != nil {
		return nil, fmt.Errorf("failed to create organization namespace: %w", err)
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get request from context: %w", err)
	}

	// Organizations created by system:masters shouldn't have a policy binding
	// or organization membership created because they're creating the organization
	// for another user in the system. It's expected those organizations will
	// create the necessary policy binding and organization membership to provide
	// the user access.
	//
	// TODO: Convert this to use a SubjectAccessReview to check if the user has
	//       permission to create an organization without a policy binding or
	//       organization membership.
	if slices.Contains(req.UserInfo.Groups, "system:masters") {
		return nil, nil
	}

	// Look up the user in the iam API
	user, err := v.lookupUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup user: %w", err)
	}

	if err := v.createOwnerPolicyBinding(ctx, org, user); err != nil {
		return nil, fmt.Errorf("failed to create owner policy binding: %w", err)
	}

	// Create OrganizationMembership for the organization owner
	if err := v.createOrganizationMembership(ctx, org, user); err != nil {
		return nil, fmt.Errorf("failed to create organization membership: %w", err)
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
func (v *OrganizationValidator) lookupUser(ctx context.Context) (*iamv1alpha1.User, error) {
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get request from context: %w", err)
	}

	// TODO: Determine if we can actually use the UID from the User object in
	//       the UserInfo of the request. Likely need to configure the OIDC
	//       authorization to map the UID from the JWT claims.
	foundUser := &iamv1alpha1.User{}
	if err := v.client.Get(ctx, client.ObjectKey{Name: req.UserInfo.Username}, foundUser); err != nil {
		return nil, fmt.Errorf("failed to get user '%s' from iam.miloapis.com API: %w", req.UserInfo.Username, err)
	}

	return foundUser, nil
}

// createOwnerPolicyBinding creates a PolicyBinding for the organization owner
func (v *OrganizationValidator) createOwnerPolicyBinding(ctx context.Context, org *resourcemanagerv1alpha1.Organization, user *iamv1alpha1.User) error {
	organizationlog.Info("Attempting to create PolicyBinding for new organization", "organization", org.Name)

	// Build the PolicyBinding
	policyBinding := &iamv1alpha1.PolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			// Don't worry about uniqueness here because the namespace will have just
			// been created for the organization.
			Name:      "organization-owner",
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
				APIGroup: resourcemanagerv1alpha1.GroupVersion.Group,
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
func (v *OrganizationValidator) createOrganizationNamespace(ctx context.Context, org *resourcemanagerv1alpha1.Organization) error {
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

func (v *OrganizationValidator) createOrganizationMembership(ctx context.Context, org *resourcemanagerv1alpha1.Organization, user *iamv1alpha1.User) error {
	organizationlog.Info("Creating OrganizationMembership for organization owner", "organization", org.Name)

	// Build the OrganizationMembership object
	organizationMembership := &resourcemanagerv1alpha1.OrganizationMembership{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("member-%s", user.Name),
			Namespace: fmt.Sprintf("organization-%s", org.Name),
		},
		Spec: resourcemanagerv1alpha1.OrganizationMembershipSpec{
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{
				Name: org.Name,
			},
			UserRef: resourcemanagerv1alpha1.MemberReference{
				Name: user.Name,
			},
		},
	}

	if err := v.client.Create(ctx, organizationMembership); err != nil {
		return fmt.Errorf("failed to create organization membership resource: %w", err)
	}

	return nil
}
