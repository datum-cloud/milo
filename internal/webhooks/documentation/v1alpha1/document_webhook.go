package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ctrl "sigs.k8s.io/controller-runtime"

	documentationv1alpha1 "go.miloapis.com/milo/pkg/apis/documentation/v1alpha1"
)

var documentLog = logf.Log.WithName("documentation-resource").WithName("document")

// SetupDocumentWebhooksWithManager sets up the webhooks for the Document resource.
func SetupDocumentWebhooksWithManager(mgr ctrl.Manager) error {
	documentLog.Info("Setting up documentation.miloapis.com documentation webhooks")

	return ctrl.NewWebhookManagedBy(mgr).
		For(&documentationv1alpha1.Document{}).
		WithValidator(&DocumentValidator{
			Client: mgr.GetClient(),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-documentation-miloapis-com-v1alpha1-document,mutating=false,failurePolicy=fail,sideEffects=None,groups=documentation.miloapis.com,resources=documents,verbs=delete,versions=v1alpha1,name=vdocument.documentation.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

type DocumentValidator struct {
	Client client.Client
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *DocumentValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *DocumentValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *DocumentValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	document, ok := obj.(*documentationv1alpha1.Document)
	if !ok {
		documentLog.Error(fmt.Errorf("failed to cast object to Document"), "failed to cast object to Document")
		return nil, errors.NewInternalError(fmt.Errorf("failed to cast object to Document"))
	}

	if document.Status.LatestRevisionRef != nil {
		documentLog.Info("Rejecting delete; related revisions exist", "namespace", document.Namespace, "name", document.Name)
		return nil, errors.NewBadRequest("cannot delete Document. It has related revision/s.")
	}

	return nil, nil
}
