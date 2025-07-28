package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var userdeactivationlog = logf.Log.WithName("userdeactivation-resource")

func SetupUserDeactivationWebhooksWithManager(mgr ctrl.Manager, systemNamespace string) error {
	userdeactivationlog.Info("Setting up iam.miloapis.com userdeactivation webhooks")

	return ctrl.NewWebhookManagedBy(mgr).
		For(&iamv1alpha1.UserDeactivation{}).
		WithDefaulter(&UserDeactivationMutator{}).
		WithValidator(&UserDeactivationValidator{
			client:          mgr.GetClient(),
			systemNamespace: systemNamespace,
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-iam-miloapis-com-v1alpha1-userdeactivation,mutating=true,failurePolicy=fail,sideEffects=None,groups=iam.miloapis.com,resources=userdeactivations,verbs=create,versions=v1alpha1,name=muserdeactivation.iam.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

// UserDeactivationMutator sets default values on UserDeactivation resources.
type UserDeactivationMutator struct{}

// Default sets the deactivatedBy field to the username of the requesting user if it is not already set.
func (m *UserDeactivationMutator) Default(ctx context.Context, obj runtime.Object) error {
	ud, ok := obj.(*iamv1alpha1.UserDeactivation)
	if !ok {
		return fmt.Errorf("failed to cast object to UserDeactivation")
	}
	userdeactivationlog.Info("Defaulting UserDeactivation", "name", ud.GetName())

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		userdeactivationlog.Error(err, "failed to get admission request from context", "name", ud.GetName())
		return fmt.Errorf("failed to get request from context: %w", err)
	}

	// Populate the field with the username present in the access token / UserInfo.
	ud.Spec.DeactivatedBy = req.UserInfo.Username

	userdeactivationlog.Info("Defaulting deactivatedBy complete", "name", ud.GetName(), "deactivatedBy", ud.Spec.DeactivatedBy)

	return nil
}

// +kubebuilder:webhook:path=/validate-iam-miloapis-com-v1alpha1-userdeactivation,mutating=false,failurePolicy=fail,sideEffects=None,groups=iam.miloapis.com,resources=userdeactivations,verbs=create,versions=v1alpha1,name=vuserdeactivation.iam.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

type UserDeactivationValidator struct {
	client          client.Client
	systemNamespace string
}

func (v *UserDeactivationValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	userDeactivation := obj.(*iamv1alpha1.UserDeactivation)
	userdeactivationlog.Info("Validating UserDeactivation", "name", userDeactivation.Name)

	var errs field.ErrorList

	userName := userDeactivation.Spec.UserRef.Name
	if userName == "" {
		errs = append(errs, field.Required(field.NewPath("spec").Child("userRef").Child("name"), "userRef.name is required"))
	}

	if len(errs) > 0 {
		return nil, errors.NewInvalid(iamv1alpha1.SchemeGroupVersion.WithKind("UserDeactivation").GroupKind(), userDeactivation.Name, errs)
	}

	// Validate that the referenced user exists
	user := &iamv1alpha1.User{}
	err := v.client.Get(ctx, client.ObjectKey{Name: userName}, user)
	if errors.IsNotFound(err) {
		userdeactivationlog.Error(err, "referenced user does not exist", "userName", userName)
		errs = append(errs, field.NotFound(field.NewPath("spec").Child("userRef").Child("name"), userName))
		return nil, errors.NewNotFound(iamv1alpha1.SchemeGroupVersion.WithResource("users").GroupResource(), userName)
	} else if err != nil {
		userdeactivationlog.Error(err, "failed to validate user reference", "userName", userName)
		return nil, errors.NewInvalid(iamv1alpha1.SchemeGroupVersion.WithKind("UserDeactivation").GroupKind(), userDeactivation.Name, errs)
	}

	// Ensure there is no existing UserDeactivation for the same user
	var existingUDList iamv1alpha1.UserDeactivationList
	if err := v.client.List(ctx, &existingUDList); err != nil {
		userdeactivationlog.Error(err, "failed to list existing UserDeactivations", "userName", userName)
		return nil, fmt.Errorf("failed to list existing UserDeactivations for user '%s': %w", userName, err)
	}
	for _, existing := range existingUDList.Items {
		if existing.Spec.UserRef.Name == userName && existing.DeletionTimestamp == nil {
			userdeactivationlog.Error(fmt.Errorf("a UserDeactivation already exists for user '%s'", userName), "existing UserDeactivation", "name", existing.Name)
			return nil, errors.NewAlreadyExists(iamv1alpha1.SchemeGroupVersion.WithResource("userdeactivations").GroupResource(), userDeactivation.Name)
		}
	}

	userdeactivationlog.Info("User reference validation successful", "userName", userName)

	return nil, nil
}

func (v *UserDeactivationValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *UserDeactivationValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
