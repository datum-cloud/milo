package v1alpha1

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
)

const platformAccessRejectionIndexKey = "iam.miloapis.com/platformaccessrejection"

func SetupPlatformAccessRejectionWebhooksWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&iamv1alpha1.PlatformAccessRejection{}).
		WithDefaulter(&PlatformAccessRejectionMutator{
			client: mgr.GetClient(),
		}).
		WithValidator(&PlatformAccessRejectionValidator{
			client: mgr.GetClient(),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-iam-miloapis-com-v1alpha1-platformaccessarejection,mutating=true,failurePolicy=fail,sideEffects=None,groups=iam.miloapis.com,resources=platformaccessarejections,verbs=create,versions=v1alpha1,name=mplatformaccessrejection.iam.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

// PlatformAccessRejectionMutator mutates PlatformAccessRejection resources to set the approver to the user who is approving the access request.
type PlatformAccessRejectionMutator struct {
	client client.Client
}

func (m *PlatformAccessRejectionMutator) Default(ctx context.Context, obj runtime.Object) error {
	return nil
}

// +kubebuilder:webhook:path=/validate-iam-miloapis-com-v1alpha1-platformaccessarejection,mutating=false,failurePolicy=fail,sideEffects=None,groups=iam.miloapis.com,resources=platformaccessarejections,verbs=create,versions=v1alpha1,name=vplatformaccessrejection.iam.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

// PlatformAccessRejectionValidator validates PlatformAccessRejection resources.
type PlatformAccessRejectionValidator struct {
	client client.Client
}

func (v *PlatformAccessRejectionValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *PlatformAccessRejectionValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *PlatformAccessRejectionValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
