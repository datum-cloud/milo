package v1alpha1

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ctrl "sigs.k8s.io/controller-runtime"

	version "go.miloapis.com/milo/pkg/version"

	documentationv1alpha1 "go.miloapis.com/milo/pkg/apis/documentation/v1alpha1"
)

var drLog = logf.Log.WithName("documentation-resource").WithName("documentrevision")

// SetupDocumentWebhooksWithManager sets up the webhooks for the Document resource.
func SetupDocumentRevisionWebhooksWithManager(mgr ctrl.Manager) error {
	drLog.Info("Setting up documentation.miloapis.com document revision webhooks")

	return ctrl.NewWebhookManagedBy(mgr).
		For(&documentationv1alpha1.DocumentRevision{}).
		WithValidator(&DocumentRevisionValidator{
			Client: mgr.GetClient(),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-documentation-miloapis-com-v1alpha1-documentation,mutating=false,failurePolicy=fail,sideEffects=None,groups=documentation.miloapis.com,resources=documentrevisions,verbs=delete;create,versions=v1alpha1,name=vdocumentrevision.documentation.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

type DocumentRevisionValidator struct {
	Client client.Client
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *DocumentRevisionValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	dr, ok := obj.(*documentationv1alpha1.DocumentRevision)
	if !ok {
		drLog.Error(fmt.Errorf("failed to cast object to DocumentRevision"), "failed to cast object to DocumentRevision")
		return nil, errors.NewInternalError(fmt.Errorf("failed to cast object to DocumentRevision"))
	}
	drLog.Info("Validating DocumentRevision", "name", dr.Name)

	var errs field.ErrorList

	// Referenced Document must exist
	document := &documentationv1alpha1.Document{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: dr.Spec.DocumentRef.Namespace, Name: dr.Spec.DocumentRef.Name}, document)
	if err != nil {
		if errors.IsNotFound(err) {
			drLog.Info("Document not found", "namespace", dr.Spec.DocumentRef.Namespace, "name", dr.Spec.DocumentRef.Name)
			errs = append(errs, field.NotFound(field.NewPath("spec", "documentRef"), dr.Spec.DocumentRef.Name))
		} else {
			drLog.Error(err, "failed to get document", "namespace", dr.Spec.DocumentRef.Namespace, "name", dr.Spec.DocumentRef.Name)
			return nil, errors.NewInternalError(err)
		}
	}

	// Version must be higher than the latest referenced revision version
	if err == nil && document.Status.LatestRevisionRef != nil {
		higher, cmpErr := version.IsVersionHigher(dr.Spec.Version, document.Status.LatestRevisionRef.Version)
		if cmpErr != nil {
			drLog.Error(cmpErr, "failed to compare versions", "namespace", dr.Spec.DocumentRef.Namespace, "name", dr.Spec.DocumentRef.Name, "version", dr.Spec.Version, "latestRevisionVersion", document.Status.LatestRevisionRef.Version)
			return nil, errors.NewInternalError(cmpErr)
		}
		if !higher {
			drLog.Info("Document revision version is not higher than the latest revision version", "namespace", dr.Spec.DocumentRef.Namespace, "name", dr.Spec.DocumentRef.Name, "version", dr.Spec.Version, "latestRevisionVersion", document.Status.LatestRevisionRef.Version)
			errs = append(errs, field.Invalid(field.NewPath("spec", "version"), dr.Spec.Version, "Document revision version is not higher than the latest referenced revision version"))
		}
	}

	// EffectiveDate must be in the future
	if !dr.Spec.EffectiveDate.Time.After(time.Now()) {
		drLog.Info("EffectiveDate is not in the future", "effectiveDate", dr.Spec.EffectiveDate.Time)
		errs = append(errs, field.Invalid(field.NewPath("spec", "effectiveDate"), dr.Spec.EffectiveDate, "EffectiveDate must be in the future"))
	}

	if len(errs) > 0 {
		invalidErr := errors.NewInvalid(documentationv1alpha1.SchemeGroupVersion.WithKind("DocumentRevision").GroupKind(), dr.Name, errs)
		drLog.Error(invalidErr, "invalid document revision")
		return nil, invalidErr
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *DocumentRevisionValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	// Update is not allowed as it is immutable at API level
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *DocumentRevisionValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, errors.NewMethodNotSupported(documentationv1alpha1.SchemeGroupVersion.WithResource("DocumentRevision").GroupResource(), "delete")
}
