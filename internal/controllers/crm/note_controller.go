package crm

import (
	"context"
	"fmt"

	crmv1alpha1 "go.miloapis.com/milo/pkg/apis/crm/v1alpha1"
	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	noteReadyConditionType   = "Ready"
	noteReadyConditionReason = "Reconciled"
)

// NoteController reconciles a Note object
type NoteController struct {
	Client client.Client
}

// +kubebuilder:rbac:groups=crm.miloapis.com,resources=notes,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=crm.miloapis.com,resources=notes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=users,verbs=get;list;watch
// +kubebuilder:rbac:groups=notification.miloapis.com,resources=contacts,verbs=get;list;watch

// Reconcile is the main reconciliation loop for the NoteController.
func (r *NoteController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("note-controllerr").WithValues("note", req.Name)

	note := &crmv1alpha1.Note{}
	if err := r.Client.Get(ctx, req.NamespacedName, note); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("reconciling Note", "note", note.Name)

	if !note.DeletionTimestamp.IsZero() {
		log.Info("Note is being deleted, skipping reconciliation", "note", note.Name)
		return ctrl.Result{}, nil
	}

	// Check if the referenced Contact exists, and delete the Note if it doesn't.
	// Garbage collection, as a Contact cannot own an Note (contact is namespaced, note is cluster scoped)
	if deleted, err := r.deleteNoteIfContactMissing(ctx, note); err != nil {
		log.Error(err, "failed to check/delete note for missing contact")
		return ctrl.Result{}, err
	} else if deleted {
		return ctrl.Result{}, nil
	}

	noteCreator := &iamv1alpha1.User{}
	// User is cluster scoped, so we only use the name
	if err := r.Client.Get(ctx, types.NamespacedName{Name: note.Spec.CreatorRef.Name}, noteCreator); err != nil {
		// This is just an edge case. If the creator user updates their email, we proceed to update the CreatedBy field.
		if apierrors.IsNotFound(err) {
			// User not found, we can't update the CreatedBy field.
			log.Info("User referenced in CreatorRef not found, status.CreatedBy will not be updated", "user", note.Spec.CreatorRef.Name)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get User", "user", note.Spec.CreatorRef.Name)
		return ctrl.Result{}, fmt.Errorf("failed to get User: %w", err)
	}

	oldNoteStatus := note.Status.DeepCopy()

	// Status will be updated on first reconciliation.
	// Webhook ensures that the Creator user exists
	note.Status.CreatedBy = noteCreator.Spec.Email
	meta.SetStatusCondition(&note.Status.Conditions, metav1.Condition{
		Type:               noteReadyConditionType,
		Status:             metav1.ConditionTrue,
		Reason:             noteReadyConditionReason,
		Message:            "Reconciled successfully",
		LastTransitionTime: metav1.Now(),
	})

	if !equality.Semantic.DeepEqual(oldNoteStatus, &note.Status) {
		log.Info("Updating Note status")
		if err := r.Client.Status().Update(ctx, note); err != nil {
			log.Error(err, "Failed to update Note status")
			return ctrl.Result{}, fmt.Errorf("failed to update Note status: %w", err)
		}
	} else {
		log.Info("Note status unchanged, skipping update")
	}

	return ctrl.Result{}, nil
}

// deleteNoteIfContactMissing checks if the referenced Contact exists.
// If it doesn't exist, it deletes the Note.
// Returns true if the Note was deleted, false otherwise.
func (r *NoteController) deleteNoteIfContactMissing(ctx context.Context, note *crmv1alpha1.Note) (bool, error) {
	log := log.FromContext(ctx)

	// Only check if the subject is a Contact
	if note.Spec.SubjectRef.Kind != "Contact" {
		return false, nil
	}

	contact := &notificationv1alpha1.Contact{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: note.Spec.SubjectRef.Name, Namespace: note.Spec.SubjectRef.Namespace}, contact)
	if err == nil {
		// Contact exists. However, if it is in the process of being deleted,
		// we should treat it as missing so that the Note is cleaned up promptly.
		if !contact.DeletionTimestamp.IsZero() {
			log.Info("Contact referenced in Note is being deleted, deleting Note", "contact", contact.Name, "namespace", contact.Namespace)
		} else {
			// Contact exists and is not being deleted.
			return false, nil
		}
	} else if !apierrors.IsNotFound(err) {
		// An error occurred that wasn't NotFound
		return false, err
	} else {
		// Contact truly not found
		log.Info("Contact referenced in Note not found, deleting Note", "contact", note.Spec.SubjectRef.Name, "namespace", note.Spec.SubjectRef.Namespace)
	}

	// At this point, the Contact either does not exist or is being deleted:
	// delete the Note to avoid dangling references.
	if err := r.Client.Delete(ctx, note); err != nil {
		return false, client.IgnoreNotFound(err)
	}

	return true, nil
}

// findNotesForContact finds all Notes that reference the given Contact as their subject.
// It is used to enqueue Note reconciliations when a Contact is deleted so that
// deleteNoteIfContactMissing can promptly clean up dangling Notes.
func (r *NoteController) findNotesForDeletedContact(ctx context.Context, obj client.Object) []reconcile.Request {
	log := log.FromContext(ctx).WithName("find-notes-for-contact").WithValues("contact", obj.GetName())

	contact, ok := obj.(*notificationv1alpha1.Contact)
	if !ok {
		log.Error(fmt.Errorf("unexpected object type %T, expected *notificationv1alpha1.Contact", obj), "unexpected object type")
		return nil
	}

	log.Info("Contact is being deleted, finding Notes that reference it")

	var notes crmv1alpha1.NoteList
	if err := r.Client.List(ctx, &notes); err != nil {
		log.Error(err, "failed to list Notes when handling Contact deletion", "contact", contact.Name, "namespace", contact.Namespace)
		return nil
	}

	var requests []reconcile.Request
	for _, note := range notes.Items {
		if note.Spec.SubjectRef.APIGroup == "notification.miloapis.com" &&
			note.Spec.SubjectRef.Kind == "Contact" &&
			note.Spec.SubjectRef.Name == contact.Name &&
			note.Spec.SubjectRef.Namespace == contact.Namespace {
			log.Info("enqueueing Note for reconciliation due to Contact deletion", "note", note.Name, "contact", contact.Name, "namespace", contact.Namespace)
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: note.Name, // Note is cluster-scoped
				},
			})
		}
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *NoteController) SetupWithManager(mgr ctrl.Manager) error {
	// Updating CreatedBy field is low priority, and likely will never happend.
	// We don't need to watch for changes to Users Email Address.
	// When the controller restarts, it will reconcile all Notes and update the CreatedBy field.

	return ctrl.NewControllerManagedBy(mgr).
		For(&crmv1alpha1.Note{}).
		Watches(&notificationv1alpha1.Contact{},
			handler.EnqueueRequestsFromMapFunc(r.findNotesForDeletedContact),
			builder.WithPredicates(predicate.Funcs{
				DeleteFunc:  func(e event.DeleteEvent) bool { return true },
				CreateFunc:  func(e event.CreateEvent) bool { return false },
				UpdateFunc:  func(e event.UpdateEvent) bool { return false },
				GenericFunc: func(e event.GenericEvent) bool { return false },
			})).
		Named("note").
		Complete(r)
}
