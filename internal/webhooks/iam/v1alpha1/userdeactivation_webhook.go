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
		WithValidator(&UserDeactivationValidator{
			client:          mgr.GetClient(),
			systemNamespace: systemNamespace,
		}).
		Complete()
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

	user := &iamv1alpha1.User{}
	err := v.client.Get(ctx, client.ObjectKey{Name: userName}, user)
	if errors.IsNotFound(err) {
		userdeactivationlog.Error(err, "referenced user does not exist", "userName", userName)
		return nil, fmt.Errorf("referenced user '%s' does not exist", userName)
	} else if err != nil {
		userdeactivationlog.Error(err, "failed to validate user reference", "userName", userName)
		return nil, fmt.Errorf("failed to validate user reference '%s': %w", userName, err)
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
