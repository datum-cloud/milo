package v1alpha1

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
)

// log is for logging in this package.
var userlog = logf.Log.WithName("user-resource")

// +kubebuilder:webhook:path=/validate-iam-miloapis-com-v1alpha1-user,mutating=false,failurePolicy=fail,sideEffects=NoneOnDryRun,groups=iam.miloapis.com,resources=users,verbs=create,versions=v1alpha1,name=vuser.iam.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

// SetupWebhooksWithManager sets up all iam.miloapis.com webhooks
func SetupUserWebhooksWithManager(mgr ctrl.Manager, systemNamespace string, userSelfManageRoleName string) error {
	userlog.Info("Setting up iam.miloapis.com user webhooks")

	return ctrl.NewWebhookManagedBy(mgr).
		For(&iamv1alpha1.User{}).
		WithValidator(&UserValidator{
			client:                 mgr.GetClient(),
			systemNamespace:        systemNamespace,
			userSelfManageRoleName: userSelfManageRoleName,
		}).
		Complete()
}

// UserValidator validates Users
type UserValidator struct {
	client                 client.Client
	decoder                admission.Decoder
	systemNamespace        string
	userSelfManageRoleName string
}

func (v *UserValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	user := obj.(*iamv1alpha1.User)
	userlog.Info("Validating User", "name", user.Name)

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get request from context: %w", err)
	}

	if req.DryRun != nil && *req.DryRun {
		return nil, nil
	}

	if err := v.createSelfManagePolicyBinding(ctx, user); err != nil {
		userlog.Error(err, "Failed to create owner policy binding")
		return nil, fmt.Errorf("failed to create owner policy binding: %w", err)
	}

	userPreferences, err := v.createUserPreference(ctx, user)
	if err != nil {
		userlog.Error(err, "Failed to create user preference")
		return nil, fmt.Errorf("failed to create user preference: %w", err)
	}

	if err := v.createUserPreferencePolicyBinding(ctx, user, userPreferences); err != nil {
		userlog.Error(err, "Failed to create user preference policy binding")
		return nil, fmt.Errorf("failed to create user preference policy binding: %w", err)
	}

	return nil, nil
}

func (v *UserValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *UserValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// createSelfManagePolicyBinding creates a PolicyBinding for the organization owner
func (v *UserValidator) createSelfManagePolicyBinding(ctx context.Context, user *iamv1alpha1.User) error {
	userlog.Info("Attempting to create PolicyBinding for new user", "user", user.Name)

	// Build the PolicyBinding
	policyBinding := &iamv1alpha1.PolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("user-self-manage-%s", user.Name),
			Namespace: v.systemNamespace,
		},
		Spec: iamv1alpha1.PolicyBindingSpec{
			RoleRef: iamv1alpha1.RoleReference{
				Name:      v.userSelfManageRoleName,
				Namespace: v.systemNamespace,
			},
			Subjects: []iamv1alpha1.Subject{
				{
					Kind: "User",
					Name: user.Name,
					UID:  string(user.GetUID()),
				},
			},
			ResourceSelector: iamv1alpha1.ResourceSelector{
				ResourceRef: &iamv1alpha1.ResourceReference{
					APIGroup: iamv1alpha1.SchemeGroupVersion.Group,
					Kind:     "User",
					Name:     user.Name,
					UID:      string(user.GetUID()),
				},
			},
		},
	}

	if err := v.client.Create(ctx, policyBinding); err != nil {
		return fmt.Errorf("failed to create policy binding resource: %w", err)
	}

	return nil
}

// createUserPreference creates a UserPreference for the new user
func (v *UserValidator) createUserPreference(ctx context.Context, user *iamv1alpha1.User) (*iamv1alpha1.UserPreference, error) {
	userlog.Info("Attempting to create UserPreference for new user", "user", user.Name)

	// Build the UserPreference
	userPreference := &iamv1alpha1.UserPreference{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("userpreference-%s", user.Name),
		},
		Spec: iamv1alpha1.UserPreferenceSpec{
			UserRef: iamv1alpha1.UserReference{
				Name: user.Name,
			},
			Theme: "system", // Default theme
		},
	}

	if err := v.client.Create(ctx, userPreference); err != nil {
		return nil, fmt.Errorf("failed to create user preference resource: %w", err)
	}

	return userPreference, nil
}

// createUserPreferencePolicyBinding creates a PolicyBinding for the user's UserPreference
func (v *UserValidator) createUserPreferencePolicyBinding(ctx context.Context, user *iamv1alpha1.User, userPreference *iamv1alpha1.UserPreference) error {
	userlog.Info("Attempting to create PolicyBinding for user preference", "user", user.Name)

	// Build the PolicyBinding
	policyBinding := &iamv1alpha1.PolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("userpreference-self-manage-%s", user.Name),
			Namespace: v.systemNamespace,
		},
		Spec: iamv1alpha1.PolicyBindingSpec{
			RoleRef: iamv1alpha1.RoleReference{
				Name:      "iam-user-preferences-manager",
				Namespace: v.systemNamespace,
			},
			Subjects: []iamv1alpha1.Subject{
				{
					Kind: "User",
					Name: user.Name,
					UID:  string(user.GetUID()),
				},
			},
			ResourceSelector: iamv1alpha1.ResourceSelector{
				ResourceRef: &iamv1alpha1.ResourceReference{
					APIGroup: iamv1alpha1.SchemeGroupVersion.Group,
					Kind:     "UserPreference",
					Name:     userPreference.Name,
					UID:      string(userPreference.UID),
				},
			},
		},
	}

	if err := v.client.Create(ctx, policyBinding); err != nil {
		return fmt.Errorf("failed to create user preference policy binding resource: %w", err)
	}

	return nil
}
