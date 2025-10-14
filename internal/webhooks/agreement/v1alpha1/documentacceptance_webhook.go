package v1alpha1

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ctrl "sigs.k8s.io/controller-runtime"

	agreementv1alpha1 "go.miloapis.com/milo/pkg/apis/agreement/v1alpha1"
	documentationv1alpha1 "go.miloapis.com/milo/pkg/apis/documentation/v1alpha1"
	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
)

var daLog = logf.Log.WithName("agreement-resource").WithName("documentacceptance")

// SetupDocumentAcceptanceWebhooksWithManager sets up the webhooks for the DocumentAcceptance resource.
func SetupDocumentAcceptanceWebhooksWithManager(mgr ctrl.Manager) error {
	daLog.Info("Setting up agreement.miloapis.com documentacceptance webhooks")

	return ctrl.NewWebhookManagedBy(mgr).
		For(&agreementv1alpha1.DocumentAcceptance{}).
		WithValidator(&DocumentAcceptanceValidator{
			Client: mgr.GetClient(),
		}).
		Complete()
}

type DocumentAcceptanceValidator struct {
	Client client.Client
}

// +kubebuilder:webhook:path=/validate-agreement-miloapis-com-v1alpha1-documentacceptance,mutating=false,failurePolicy=fail,sideEffects=None,groups=agreement.miloapis.com,resources=documentacceptances,verbs=delete;create,versions=v1alpha1,name=vdocumentacceptance.agreement.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

func (r *DocumentAcceptanceValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	da, ok := obj.(*agreementv1alpha1.DocumentAcceptance)
	if !ok {
		daLog.Error(fmt.Errorf("failed to cast object to DocumentAcceptance"), "failed to cast object to DocumentAcceptance")
		return nil, errors.NewInternalError(fmt.Errorf("failed to cast object to DocumentAcceptance"))
	}
	daLog.Info("Validating DocumentAcceptance", "name", da.Name)

	var errs field.ErrorList

	// Referenced DocumentRevision must exist
	documentRevision := &documentationv1alpha1.DocumentRevision{}
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: da.Spec.DocumentRevisionRef.Namespace, Name: da.Spec.DocumentRevisionRef.Name}, documentRevision); err != nil {
		if errors.IsNotFound(err) {
			errs = append(errs, field.NotFound(field.NewPath("spec", "documentRevisionRef"), da.Spec.DocumentRevisionRef.Name))
			// Further validations cannot be done with incorrrect document revision
			return nil, errors.NewInvalid(agreementv1alpha1.SchemeGroupVersion.WithKind("DocumentAcceptance").GroupKind(), da.Name, errs)
		} else {
			daLog.Error(err, "failed to get DocumentRevision", "namespace", da.Spec.DocumentRevisionRef.Namespace, "name", da.Spec.DocumentRevisionRef.Name)
			return nil, errors.NewInternalError(err)
		}
	}

	// Validate correct DocumentRevision version
	if da.Spec.DocumentRevisionRef.Version != documentRevision.Spec.Version {
		errs = append(errs, field.Invalid(field.NewPath("spec", "documentRevisionRef", "version"), da.Spec.DocumentRevisionRef.Version, "documentRevisionRef version must match the referenced document revision version"))
	}

	// Validate expected subject kind
	daSubjRefKind := &documentationv1alpha1.DocumentRevisionExpectedSubjectKind{
		APIGroup: da.Spec.SubjectRef.APIGroup,
		Kind:     da.Spec.SubjectRef.Kind,
	}
	if !slices.Contains(documentRevision.Spec.ExpectedSubjectKinds, *daSubjRefKind) {
		errs = append(errs, field.Invalid(field.NewPath("spec", "subjectRef"), da.Spec.SubjectRef, "subjectRef must be one of the expected subject kinds"))
	}

	// Validate expected accepter kind
	daAccepterRef := da.Spec.AccepterRef
	daAccepterKind := &documentationv1alpha1.DocumentRevisionExpectedAccepterKind{
		APIGroup: daAccepterRef.APIGroup,
		Kind:     daAccepterRef.Kind,
	}
	if !slices.Contains(documentRevision.Spec.ExpectedAccepterKinds, *daAccepterKind) {
		errs = append(errs, field.Invalid(field.NewPath("spec", "accepterRef"), daAccepterRef, "accepterRef must be one of the expected accepter kinds"))
	}

	// Validate accepter reference
	var accepterObj client.Object
	switch daAccepterRef.Kind {
	case "User":
		accepterObj = &iamv1alpha1.User{}
	case "MachineAccount":
		accepterObj = &iamv1alpha1.MachineAccount{}
	default:
		// Should never happen, but just in case
		errs = append(errs, field.Invalid(field.NewPath("spec", "accepterRef", "kind"), daAccepterRef.Kind, "missing backend validation for kind"))
	}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: daAccepterRef.Name, Namespace: daAccepterRef.Namespace}, accepterObj); err != nil {
		if errors.IsNotFound(err) {
			errs = append(errs, field.NotFound(field.NewPath("spec", "accepterRef", "name"), daAccepterRef.Name))
		} else {
			daLog.Error(err, "failed to get accepter", "namespace", daAccepterRef.Namespace, "name", daAccepterRef.Name)
			return nil, errors.NewInternalError(err)
		}
	}

	if len(errs) > 0 {
		invalidErr := errors.NewInvalid(agreementv1alpha1.SchemeGroupVersion.WithKind("DocumentAcceptance").GroupKind(), da.Name, errs)
		daLog.Error(invalidErr, "invalid document acceptance")
		return nil, invalidErr
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *DocumentAcceptanceValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, errors.NewMethodNotSupported(agreementv1alpha1.SchemeGroupVersion.WithResource("documentacceptances").GroupResource(), "update")
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *DocumentAcceptanceValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, errors.NewMethodNotSupported(agreementv1alpha1.SchemeGroupVersion.WithResource("documentacceptances").GroupResource(), "delete")
}
