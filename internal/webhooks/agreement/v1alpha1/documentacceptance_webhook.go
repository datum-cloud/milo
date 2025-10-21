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
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
)

var daLog = logf.Log.WithName("agreement-resource").WithName("documentacceptance")

// daIndexKey is the key used to index DocumentAcceptance by .spec.documentRevisionRef and .spec.subjectRef
const daIndexKey = "agreement.miloapis.com/documentacceptance-index"

// buildDaIndexKey returns the composite key used for indexing DocumentAcceptance by .spec.documentRevisionRef and .spec.subjectRef
func buildDaIndexKey(da agreementv1alpha1.DocumentAcceptance) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		da.Spec.DocumentRevisionRef.Name, da.Spec.DocumentRevisionRef.Namespace, da.Spec.DocumentRevisionRef.Version,
		da.Spec.SubjectRef.Name, da.Spec.SubjectRef.Namespace, da.Spec.SubjectRef.APIGroup, da.Spec.SubjectRef.Kind)
}

// SetupDocumentAcceptanceWebhooksWithManager sets up the webhooks for the DocumentAcceptance resource.
func SetupDocumentAcceptanceWebhooksWithManager(mgr ctrl.Manager) error {
	daLog.Info("Setting up agreement.miloapis.com documentacceptance webhooks")

	if err := mgr.GetFieldIndexer().IndexField(context.Background(),
		&agreementv1alpha1.DocumentAcceptance{}, daIndexKey,
		func(obj client.Object) []string {
			da := obj.(*agreementv1alpha1.DocumentAcceptance)
			return []string{buildDaIndexKey(*da)}
		}); err != nil {
		return fmt.Errorf("failed to set field index on DocumentAcceptance by .spec.documentRevisionRef and .spec.subjectRef: %w", err)
	}

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

	// Check if DocumentAcceptance already exists
	existing := &agreementv1alpha1.DocumentAcceptanceList{}
	if err := r.Client.List(ctx, existing,
		client.MatchingFields{daIndexKey: buildDaIndexKey(*da)}); err != nil {
		return nil, errors.NewInternalError(err)
	}
	if len(existing.Items) > 0 {
		errs = append(errs, field.Duplicate(
			field.NewPath("spec"),
			"a DocumentAcceptance with the same documentRevisionRef and subjectRef already exists",
		))
		return nil, errors.NewInvalid(agreementv1alpha1.SchemeGroupVersion.WithKind("DocumentAcceptance").GroupKind(), da.Name, errs)
	}

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
	daSubjectRef := da.Spec.SubjectRef
	daSubjRefKind := &documentationv1alpha1.DocumentRevisionExpectedSubjectKind{
		APIGroup: da.Spec.SubjectRef.APIGroup,
		Kind:     da.Spec.SubjectRef.Kind,
	}
	if !slices.Contains(documentRevision.Spec.ExpectedSubjectKinds, *daSubjRefKind) {
		errs = append(errs, field.Invalid(field.NewPath("spec", "subjectRef"), da.Spec.SubjectRef, "subjectRef must be one of the expected subject kinds"))
	} else {
		// If the expected kind is validated, validate the subject reference
		if daSubjRefKind.APIGroup == "resourcemanager.miloapis.com" {
			var subjectObj client.Object
			switch daSubjRefKind.Kind {
			case "Organization":
				subjectObj = &resourcemanagerv1alpha1.Organization{}
			default:
				// Should never happen, but just in case
				errs = append(errs, field.Invalid(field.NewPath("spec", "subjectRef", "kind"), daSubjRefKind.Kind, "missing backend validation for kind"))
			}
			if err := r.Client.Get(ctx, client.ObjectKey{Name: daSubjectRef.Name, Namespace: daSubjectRef.Namespace}, subjectObj); err != nil {
				if errors.IsNotFound(err) {
					errs = append(errs, field.NotFound(field.NewPath("spec", "subjectRef", "name"), daSubjectRef.Name))
				} else {
					daLog.Error(err, "failed to get subject reference", "namespace", daSubjectRef.Namespace, "name", daSubjectRef.Name)
					return nil, errors.NewInternalError(err)
				}
			}
		} else {
			errs = append(errs, field.Invalid(field.NewPath("spec", "subjectRef", "apiGroup"), daSubjRefKind.APIGroup, "missing backend validation for apiGroup"))
		}
	}

	// Validate expected accepter kind
	daAccepterRef := da.Spec.AccepterRef
	daAccepterKind := &documentationv1alpha1.DocumentRevisionExpectedAccepterKind{
		APIGroup: daAccepterRef.APIGroup,
		Kind:     daAccepterRef.Kind,
	}
	if !slices.Contains(documentRevision.Spec.ExpectedAccepterKinds, *daAccepterKind) {
		errs = append(errs, field.Invalid(field.NewPath("spec", "accepterRef"), daAccepterRef, "accepterRef must be one of the expected accepter kinds"))
	} else {
		// If the expected kind is validated, validate the accepter reference
		if daAccepterRef.APIGroup == "iam.miloapis.com" {
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
		} else {
			errs = append(errs, field.Invalid(field.NewPath("spec", "accepterRef", "apiGroup"), daAccepterRef.APIGroup, "missing backend validation for apiGroup"))
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
