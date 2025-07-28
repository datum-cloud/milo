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
)

var userdeactivationlog = logf.Log.WithName("userdeactivation-resource")

// +kubebuilder:webhook:path=/validate-iam-miloapis-com-v1alpha1-userdeactivation,mutating=false,failurePolicy=fail,sideEffects=NoneOnDryRun,groups=iam.miloapis.com,resources=userdeactivations,verbs=create,versions=v1alpha1,name=vuserdeactivation.iam.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

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

type UserDeactivationValidator struct {
	client          client.Client
	systemNamespace string
}

func (v *UserDeactivationValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	userDeactivation := obj.(*iamv1alpha1.UserDeactivation)
	userdeactivationlog.Info("Validating UserDeactivation", "name", userDeactivation.Name)

	// Validate that the referenced user exists
	userName := userDeactivation.Spec.UserRef.Name
	if userName == "" {
		return nil, fmt.Errorf("userRef.name is required")
	}

	// Ensure spec.deactivatedBy matches the requesting user (it should have been defaulted by the mutator)
	req, reqErr := admission.RequestFromContext(ctx)
	if reqErr != nil {
		return nil, fmt.Errorf("failed to get request from context: %w", reqErr)
	}
	if userDeactivation.Spec.DeactivatedBy == "" {
		return nil, fmt.Errorf("spec.deactivatedBy must be set by the system")
	}
	if userDeactivation.Spec.DeactivatedBy != req.UserInfo.Username {
		return nil, fmt.Errorf("spec.deactivatedBy is managed by the system and cannot be set by the client; if provided, it must match the authenticated user")
	}

	user := &iamv1alpha1.User{}
	err := v.client.Get(ctx, client.ObjectKey{Name: userName}, user)
	if errors.IsNotFound(err) {
		userdeactivationlog.Error(err, "referenced user does not exist", "userName", userName)
		return nil, fmt.Errorf("referenced user '%s' does not exist", userName)
	} else if err != nil {
		userdeactivationlog.Error(err, "failed to validate user reference", "userName", userName)
		return nil, fmt.Errorf("failed to validate user reference '%s': %w", userName, err)
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
			return nil, fmt.Errorf("a UserDeactivation already exists for user '%s'", userName)
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
