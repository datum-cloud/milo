package v1alpha1

import (
	"context"
	"fmt"
	"slices"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var userinvitationlog = logf.Log.WithName("userinvitation-resource")

// SetupUserInvitationWebhooksWithManager sets up the webhooks for UserInvitation resources.
func SetupUserInvitationWebhooksWithManager(mgr ctrl.Manager, systemNamespace string) error {
	userinvitationlog.Info("Setting up iam.miloapis.com userinvitation webhooks")

	return ctrl.NewWebhookManagedBy(mgr).
		For(&iamv1alpha1.UserInvitation{}).
		WithDefaulter(&UserInvitationMutator{}).
		WithValidator(&UserInvitationValidator{
			client:          mgr.GetClient(),
			systemNamespace: systemNamespace,
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-iam-miloapis-com-v1alpha1-userinvitation,mutating=true,failurePolicy=fail,sideEffects=None,groups=iam.miloapis.com,resources=userinvitations,verbs=create,versions=v1alpha1,name=muserinvitation.iam.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

// UserInvitationMutator sets default values for UserInvitation resources.
type UserInvitationMutator struct{}

// Default sets the InvitedBy field to the requesting user if not already set.
func (m *UserInvitationMutator) Default(ctx context.Context, obj runtime.Object) error {
	ui, ok := obj.(*iamv1alpha1.UserInvitation)
	if !ok {
		return fmt.Errorf("failed to cast object to UserInvitation")
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		userinvitationlog.Error(err, "failed to get admission request from context", "name", ui.GetName())
		return fmt.Errorf("failed to get request from context: %w", err)
	}

	ui.Spec.InvitedBy = iamv1alpha1.UserReference{
		Name: req.UserInfo.Username,
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-iam-miloapis-com-v1alpha1-userinvitation,mutating=false,failurePolicy=fail,sideEffects=None,groups=iam.miloapis.com,resources=userinvitations,verbs=create,versions=v1alpha1,name=vuserinvitation.iam.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

// UserInvitationValidator validates UserInvitation resources.
type UserInvitationValidator struct {
	client          client.Client
	systemNamespace string
}

// ValidateCreate ensures the expiration date, if provided, is not already expired.
func (v *UserInvitationValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	ui, ok := obj.(*iamv1alpha1.UserInvitation)
	if !ok {
		return nil, fmt.Errorf("failed to cast object to UserInvitation")
	}
	userinvitationlog.Info("Validating UserInvitation", "name", ui.Name)

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		userinvitationlog.Error(err, "failed to get admission request from context", "name", ui.GetName())
		return nil, fmt.Errorf("failed to get request from context: %w", err)
	}

	var errs field.ErrorList

	// Ensure the expiration date is in the future
	if ui.Spec.ExpirationDate != nil {
		now := metav1.NewTime(time.Now().UTC())
		if ui.Spec.ExpirationDate.Before(&now) {
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("expirationDate"), ui.Spec.ExpirationDate.String(), "expirationDate must be in the future"))
		}
	}

	// Ensure the ui OrganizationRef is in the organization's namespace
	if fmt.Sprintf("organization-%s", ui.Spec.OrganizationRef.Name) != req.Namespace {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("organizationRef"), ui.Spec.OrganizationRef.Name, "organizationRef must be the same as the requesting user's organization"))
	}

	// Ensure that the roles are valid
	for i, role := range ui.Spec.Roles {
		canGetRole := true
		if role.Name == "" {
			canGetRole = false
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("roles").Index(i).Child("name"), role.Name, "name is required"))
		}
		allowedNamespaces := []string{req.Namespace, v.systemNamespace}
		if !slices.Contains(allowedNamespaces, role.Namespace) {
			canGetRole = false
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("roles").Index(i).Child("namespace"), role.Namespace, "namespace is required"))
		}
		if !canGetRole {
			continue
		}

		foundRole := &iamv1alpha1.Role{}
		if err := v.client.Get(ctx, client.ObjectKey{Name: role.Name, Namespace: role.Namespace}, foundRole); err != nil {
			if errors.IsNotFound(err) {
				errs = append(errs, field.NotFound(field.NewPath("spec").Child("roles").Index(i).Child("name"), fmt.Sprintf("%s/%s", role.Namespace, role.Name)))
				continue
			}
			userinvitationlog.Error(err, "failed to get role reference", "role", role)
			return nil, fmt.Errorf("failed to get role reference: %w", err)
		}
	}

	if len(errs) > 0 {
		return nil, errors.NewInvalid(iamv1alpha1.SchemeGroupVersion.WithKind("UserInvitation").GroupKind(), ui.Name, errs)
	}

	return nil, nil
}

func (v *UserInvitationValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *UserInvitationValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
