package v1alpha1

import (
	"context"
	"fmt"
	"time"

	crmv1alpha1 "go.miloapis.com/milo/pkg/apis/crm/v1alpha1"
	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var noteLog = logf.Log.WithName("note-resource")

// SetupNoteWebhooksWithManager sets up the webhooks for the Note resource.
func SetupNoteWebhooksWithManager(mgr ctrl.Manager) error {
	noteLog.Info("Setting up crm.miloapis.com note webhooks")
	return ctrl.NewWebhookManagedBy(mgr).
		For(&crmv1alpha1.Note{}).
		WithDefaulter(&NoteMutator{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}).
		WithValidator(&NoteValidator{
			Client: mgr.GetClient(),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-crm-miloapis-com-v1alpha1-note,mutating=true,failurePolicy=fail,sideEffects=None,groups=crm.miloapis.com,resources=notes,verbs=create,versions=v1alpha1,name=mnote.crm.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

type NoteMutator struct {
	Client client.Client
	Scheme *runtime.Scheme
}

var _ admission.CustomDefaulter = &NoteMutator{}

func (m *NoteMutator) Default(ctx context.Context, obj runtime.Object) error {
	note, ok := obj.(*crmv1alpha1.Note)
	if !ok {
		return errors.NewInternalError(fmt.Errorf("failed to cast object to Note"))
	}
	noteLog.Info("Defaulting Note", "name", note.Name)

	// Set owner reference for the note.
	// If the subject is a user, set the owner reference to the user.
	// If the subject is a contact, set the owner reference to the contact.
	if note.Spec.SubjectRef.APIGroup == "iam.miloapis.com" && note.Spec.SubjectRef.Kind == "User" {
		user := &iamv1alpha1.User{}
		if err := m.Client.Get(ctx, types.NamespacedName{Name: note.Spec.SubjectRef.Name}, user); err != nil {
			// If we fail to get user, we can't set owner.
			return errors.NewInternalError(fmt.Errorf("failed to get referenced user: %w", err))
		}
		if err := controllerutil.SetOwnerReference(user, note, m.Scheme); err != nil {
			return errors.NewInternalError(fmt.Errorf("failed to set owner reference: %w", err))
		}
	}

	if note.Spec.SubjectRef.APIGroup == "notification.miloapis.com" && note.Spec.SubjectRef.Kind == "Contact" {
		// Contact namespace is required.
		ns := note.Spec.SubjectRef.Namespace
		if ns == "" {
			// Validation will catch this, but we can't proceed with lookup without NS.
			return nil
		}

		contact := &notificationv1alpha1.Contact{}
		if err := m.Client.Get(ctx, types.NamespacedName{Name: note.Spec.SubjectRef.Name, Namespace: ns}, contact); err != nil {
			return errors.NewInternalError(fmt.Errorf("failed to get referenced contact: %w", err))
		}
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get request from context: %w", err)
	}

	// Set creator reference for the note.
	creatorUser := &iamv1alpha1.User{}
	if err := m.Client.Get(ctx, client.ObjectKey{Name: string(req.UserInfo.UID)}, creatorUser); err != nil {
		return errors.NewInternalError(fmt.Errorf("failed to get user '%s' from iam.miloapis.com API: %w", string(req.UserInfo.UID), err))
	}

	note.Spec.CreatorRef = iamv1alpha1.UserReference{
		Name: creatorUser.Name,
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-crm-miloapis-com-v1alpha1-note,mutating=false,failurePolicy=fail,sideEffects=None,groups=crm.miloapis.com,resources=notes,verbs=create;update,versions=v1alpha1,name=vnote.crm.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

type NoteValidator struct {
	Client client.Client
}

var _ admission.CustomValidator = &NoteValidator{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *NoteValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	note, ok := obj.(*crmv1alpha1.Note)
	if !ok {
		return nil, errors.NewInternalError(fmt.Errorf("failed to cast object to Note"))
	}
	noteLog.Info("Validating Note creation", "name", note.Name)

	return nil, v.validateNote(ctx, note)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *NoteValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	note, ok := newObj.(*crmv1alpha1.Note)
	if !ok {
		return nil, errors.NewInternalError(fmt.Errorf("failed to cast object to Note"))
	}
	noteLog.Info("Validating Note update", "name", note.Name)

	return nil, v.validateNote(ctx, note)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *NoteValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *NoteValidator) validateNote(ctx context.Context, note *crmv1alpha1.Note) error {
	var allErrs field.ErrorList

	// Validate NextActionTime is not in the past
	if note.Spec.NextActionTime != nil {
		if note.Spec.NextActionTime.Time.Before(time.Now()) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "nextActionTime"), note.Spec.NextActionTime, "nextActionTime cannot be in the past"))
		}
	}

	// Validate Subject
	subjectPath := field.NewPath("spec", "subject")

	switch note.Spec.SubjectRef.APIGroup {
	case "iam.miloapis.com":
		if note.Spec.SubjectRef.Kind != "User" {
			allErrs = append(allErrs, field.Invalid(subjectPath.Child("kind"), note.Spec.SubjectRef.Kind, "kind must be User for iam.miloapis.com"))
		} else {
			// Check if User exists
			user := &iamv1alpha1.User{}
			if err := v.Client.Get(ctx, types.NamespacedName{Name: note.Spec.SubjectRef.Name}, user); err != nil {
				if errors.IsNotFound(err) {
					allErrs = append(allErrs, field.NotFound(subjectPath.Child("name"), note.Spec.SubjectRef.Name))
				} else {
					return errors.NewInternalError(fmt.Errorf("failed to get user: %w", err))
				}
			}
		}
	case "notification.miloapis.com":
		if note.Spec.SubjectRef.Kind != "Contact" {
			allErrs = append(allErrs, field.Invalid(subjectPath.Child("kind"), note.Spec.SubjectRef.Kind, "kind must be Contact for notification.miloapis.com"))
		} else {
			// Check if Contact exists
			contact := &notificationv1alpha1.Contact{}

			if note.Spec.SubjectRef.Namespace == "" {
				allErrs = append(allErrs, field.Required(subjectPath.Child("namespace"), "namespace is required for Contact"))
			} else {
				if err := v.Client.Get(ctx, types.NamespacedName{Name: note.Spec.SubjectRef.Name, Namespace: note.Spec.SubjectRef.Namespace}, contact); err != nil {
					if errors.IsNotFound(err) {
						allErrs = append(allErrs, field.NotFound(subjectPath.Child("name"), note.Spec.SubjectRef.Name))
					} else {
						return errors.NewInternalError(fmt.Errorf("failed to get contact: %w", err))
					}
				}
			}
		}
	default:
		allErrs = append(allErrs, field.Invalid(subjectPath.Child("apiGroup"), note.Spec.SubjectRef.APIGroup, "apiGroup must be iam.miloapis.com or notification.miloapis.com"))
	}

	if len(allErrs) == 0 {
		return nil
	}
	return errors.NewInvalid(crmv1alpha1.SchemeGroupVersion.WithKind("Note").GroupKind(), note.Name, allErrs)
}
