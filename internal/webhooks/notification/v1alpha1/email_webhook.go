package v1alpha1

import (
	"context"
	"fmt"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	"go.miloapis.com/milo/pkg/email/templating"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var emailLog = logf.Log.WithName("email-resource")

// SetupEmailWebhooksWithManager sets up the webhooks for the Email resource.
func SetupEmailWebhooksWithManager(mgr ctrl.Manager) error {
	emailLog.Info("Setting up notification.miloapis.com email webhooks")

	return ctrl.NewWebhookManagedBy(mgr).
		For(&notificationv1alpha1.Email{}).
		WithValidator(&EmailValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-notification-miloapis-com-v1alpha1-email,mutating=false,failurePolicy=fail,sideEffects=None,groups=notification.miloapis.com,resources=emails,verbs=create;update;delete;patch,versions=v1alpha1,name=vemails.notification.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

// EmailValidator validates Email resources.
type EmailValidator struct {
	Client client.Client
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *EmailValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	email, ok := obj.(*notificationv1alpha1.Email)
	if !ok {
		return nil, fmt.Errorf("failed to cast object to Email")
	}
	emailLog.Info("validate create", "name", email.Name)

	var errs field.ErrorList

	// Validate User reference
	user := &iamv1alpha1.User{}
	err := v.Client.Get(ctx, client.ObjectKey{Name: email.Spec.UserRef.Name}, user)
	if err != nil {
		if errors.IsNotFound(err) {
			emailLog.Info("user not found", "name", email.Spec.UserRef.Name)
			errs = append(errs, field.NotFound(field.NewPath("spec", "userRef", "name"), email.Spec.UserRef.Name))
		} else {
			emailLog.Error(err, "error getting user", "name", email.Spec.UserRef.Name)
			errs = append(errs, field.InternalError(field.NewPath("spec", "userRef", "name"), fmt.Errorf("getting user: %w", err)))
		}
	}

	// Validate EmailTemplate reference
	template := &notificationv1alpha1.EmailTemplate{}
	err = v.Client.Get(ctx, client.ObjectKey{Name: email.Spec.TemplateRef.Name}, template)
	if err != nil {
		if errors.IsNotFound(err) {
			emailLog.Info("email template not found", "name", email.Spec.TemplateRef.Name)
			errs = append(errs, field.NotFound(field.NewPath("spec", "templateRef", "name"), email.Spec.TemplateRef.Name))
		} else {
			emailLog.Error(err, "error getting email template", "name", email.Spec.TemplateRef.Name)
			errs = append(errs, field.InternalError(field.NewPath("spec", "templateRef", "name"), fmt.Errorf("getting email template: %w", err)))
		}
	}

	// Validate Email variables against declared template variables
	if err == nil { // template successfully fetched
		varValidationErrs := templating.ValidateEmailVariables(email, template)
		errs = append(errs, varValidationErrs...)
	}

	if len(errs) > 0 {
		invalidErr := errors.NewInvalid(notificationv1alpha1.SchemeGroupVersion.WithKind("Email").GroupKind(), email.Name, errs)
		emailLog.Error(invalidErr, "invalid email")
		return nil, invalidErr
	}

	return nil, nil

}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
// We do not allow updates to Email resources as they are immutable.
func (v *EmailValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	_, ok := newObj.(*notificationv1alpha1.Email)
	if !ok {
		return nil, fmt.Errorf("failed to cast object to Email")
	}
	return nil, errors.NewMethodNotSupported(notificationv1alpha1.SchemeGroupVersion.WithResource("emails").GroupResource(), "update")
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
// We do not allow deletion of Email resources as we want to keep a record of all emails sent.
func (v *EmailValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	_, ok := obj.(*notificationv1alpha1.Email)
	if !ok {
		return nil, fmt.Errorf("failed to cast object to Email")
	}
	return nil, errors.NewMethodNotSupported(notificationv1alpha1.SchemeGroupVersion.WithResource("emails").GroupResource(), "delete")
}
