package v1alpha1

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var userinvitationlog = logf.Log.WithName("userinvitation-resource")

// SetupUserInvitationWebhooksWithManager sets up the webhooks for UserInvitation resources.
func SetupUserInvitationWebhooksWithManager(mgr ctrl.Manager) error {
	userinvitationlog.Info("Setting up iam.miloapis.com userinvitation webhooks")

	return ctrl.NewWebhookManagedBy(mgr).
		For(&iamv1alpha1.UserInvitation{}).
		WithDefaulter(&UserInvitationMutator{}).
		WithValidator(&UserInvitationValidator{}).
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
type UserInvitationValidator struct{}

// ValidateCreate ensures the expiration date, if provided, is not already expired.
func (v *UserInvitationValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	ui, ok := obj.(*iamv1alpha1.UserInvitation)
	if !ok {
		return nil, fmt.Errorf("failed to cast object to UserInvitation")
	}
	userinvitationlog.Info("Validating UserInvitation", "name", ui.Name)

	var errs field.ErrorList

	if ui.Spec.ExpirationDate != nil {
		now := metav1.NewTime(time.Now().UTC())
		if ui.Spec.ExpirationDate.Before(&now) {
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("expirationDate"), ui.Spec.ExpirationDate.String(), "expirationDate must be in the future"))
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
