package v1alpha1

import (
	"context"
	"fmt"
	"time"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	notesv1alpha1 "go.miloapis.com/milo/pkg/apis/notes/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var noteLog = logf.Log.WithName("note-resource")

func SetupNoteWebhooksWithManager(mgr ctrl.Manager) error {
	noteLog.Info("Setting up notes.miloapis.com note webhooks")
	return ctrl.NewWebhookManagedBy(mgr).
		For(&notesv1alpha1.Note{}).
		WithDefaulter(&NoteMutator{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}).
		WithValidator(&NoteValidator{
			Client: mgr.GetClient(),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-notes-miloapis-com-v1alpha1-note,mutating=true,failurePolicy=fail,sideEffects=None,groups=notes.miloapis.com,resources=notes,verbs=create,versions=v1alpha1,name=mnote.notes.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

type NoteMutator struct {
	Client client.Client
	Scheme *runtime.Scheme
}

var _ admission.CustomDefaulter = &NoteMutator{}

func (m *NoteMutator) Default(ctx context.Context, obj runtime.Object) error {
	note, ok := obj.(*notesv1alpha1.Note)
	if !ok {
		return errors.NewInternalError(fmt.Errorf("failed to cast object to Note"))
	}
	noteLog.Info("Defaulting Note", "name", note.Name, "namespace", note.Namespace)

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get request from context: %w", err)
	}

	creatorUser := &iamv1alpha1.User{}
	if err := m.Client.Get(ctx, client.ObjectKey{Name: string(req.UserInfo.UID)}, creatorUser); err != nil {
		return errors.NewInternalError(fmt.Errorf("failed to get user '%s' from iam.miloapis.com API: %w", string(req.UserInfo.UID), err))
	}

	note.Spec.CreatorRef = iamv1alpha1.UserReference{
		Name: creatorUser.Name,
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-notes-miloapis-com-v1alpha1-note,mutating=false,failurePolicy=fail,sideEffects=None,groups=notes.miloapis.com,resources=notes,verbs=create;update,versions=v1alpha1,name=vnote.notes.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

type NoteValidator struct {
	Client client.Client
}

var _ admission.CustomValidator = &NoteValidator{}

func (v *NoteValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	note, ok := obj.(*notesv1alpha1.Note)
	if !ok {
		return nil, errors.NewInternalError(fmt.Errorf("failed to cast object to Note"))
	}
	noteLog.Info("Validating Note creation", "name", note.Name, "namespace", note.Namespace)

	return nil, v.validateNote(ctx, note, false)
}

func (v *NoteValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	note, ok := newObj.(*notesv1alpha1.Note)
	if !ok {
		return nil, errors.NewInternalError(fmt.Errorf("failed to cast object to Note"))
	}
	oldNote, ok := oldObj.(*notesv1alpha1.Note)
	if !ok {
		return nil, errors.NewInternalError(fmt.Errorf("failed to cast old object to Note"))
	}
	noteLog.Info("Validating Note update", "name", note.Name, "namespace", note.Namespace)

	skipNextActionTimeValidation := oldNote.Spec.NextActionTime != nil &&
		note.Spec.NextActionTime != nil &&
		oldNote.Spec.NextActionTime.Time.Equal(note.Spec.NextActionTime.Time)

	return nil, v.validateNote(ctx, note, skipNextActionTimeValidation)
}

func (v *NoteValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *NoteValidator) validateNote(ctx context.Context, note *notesv1alpha1.Note, skipNextActionTimeValidation bool) error {
	var allErrs field.ErrorList

	if !skipNextActionTimeValidation && note.Spec.NextActionTime != nil {
		if note.Spec.NextActionTime.Time.Before(time.Now()) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "nextActionTime"), note.Spec.NextActionTime, "nextActionTime cannot be in the past"))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return errors.NewInvalid(notesv1alpha1.SchemeGroupVersion.WithKind("Note").GroupKind(), note.Name, allErrs)
}
