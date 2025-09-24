package v1alpha1

import (
	"context"
	"fmt"

	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var contactGroupLog = logf.Log.WithName("contactgroup-resource")

// SetupContactGroupWebhooksWithManager sets up the webhooks for the ContactGroup resource.
func SetupContactGroupWebhooksWithManager(mgr ctrl.Manager) error {
	contactGroupLog.Info("Setting up notification.miloapis.com contactgroup webhooks")

	// Index spec.displayName for efficient lookups in validator
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &notificationv1alpha1.ContactGroup{}, "spec.displayName", func(rawObj client.Object) []string {
		cg := rawObj.(*notificationv1alpha1.ContactGroup)
		if cg.Spec.DisplayName == "" {
			return nil
		}
		return []string{cg.Spec.DisplayName}
	}); err != nil {
		return fmt.Errorf("failed to set contactgroup field index: %w", err)
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&notificationv1alpha1.ContactGroup{}).
		WithValidator(&ContactGroupValidator{
			Client: mgr.GetClient(),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-notification-miloapis-com-v1alpha1-contactgroup,mutating=false,failurePolicy=fail,sideEffects=None,groups=notification.miloapis.com,resources=contactgroups,verbs=create,versions=v1alpha1,name=vcontactgroup.notification.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

type ContactGroupValidator struct {
	Client client.Client
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *ContactGroupValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	cg, ok := obj.(*notificationv1alpha1.ContactGroup)
	if !ok {
		return nil, errors.NewInternalError(fmt.Errorf("failed to cast object to ContactGroup"))
	}

	contactGroupLog.Info("Validating ContactGroup", "name", cg.Name)

	var errs field.ErrorList

	// Ensure no other ContactGroup exists in the same display name and visibility.
	var existing notificationv1alpha1.ContactGroupList
	if err := v.Client.List(ctx, &existing,
		client.MatchingFields{"spec.displayName": cg.Spec.DisplayName}); err != nil {
		return nil, errors.NewInternalError(fmt.Errorf("failed to list contactgroups: %w", err))
	}
	for _, item := range existing.Items {
		if item.Name == cg.Name {
			continue // same object during update
		}
		if item.Spec.DisplayName == cg.Spec.DisplayName && item.Spec.Visibility == cg.Spec.Visibility {
			errs = append(errs, field.Invalid(field.NewPath("spec", "displayName"), cg.Spec.DisplayName, fmt.Sprintf("a ContactGroup named %s already has this displayName and visibility", item.Name)))
			break
		}
	}

	if len(errs) > 0 {
		return nil, errors.NewInvalid(notificationv1alpha1.SchemeGroupVersion.WithKind("ContactGroup").GroupKind(), cg.Name, errs)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *ContactGroupValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *ContactGroupValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
