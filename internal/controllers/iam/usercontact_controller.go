package iam

import (
	"context"
	"fmt"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// contactEmailIndexKey is the field index key used for efficient lookups by email.
	contactEmailIndexKey = "spec.email"

	// contactSubjectNameIndexKey is the field index key used for efficient lookups by SubjectRef.Name.
	contactSubjectNameIndexKey = "spec.subject.name"

	// userContactFieldOwner is the field manager name used for server-side apply/patch operations.
	userContactFieldOwner = "user-contact-controller"

	// ContactUserSyncedCondition is the condition type that tracks sync status with a User.
	ContactUserSyncedCondition = "UserSynced"

	// ContactUserSyncedReason indicates the contact was successfully synced with a user.
	ContactUserSyncedReason = "SyncedWithUser"

	// ContactUserUnlinkedReason indicates the contact's user reference was removed.
	ContactUserUnlinkedReason = "UserUnlinked"
)

// UserContactController reconciles User objects and ensures corresponding Contacts exist.
// When a User is created or updated, it searches for existing Contacts with the same email.
// If no Contact is found, a new Contact referencing the User is created.
// If a Contact exists, it updates the Contact to reference the User and syncs the email.
//
// This controller does NOT use finalizers. Instead, it watches Contacts and cleans up
// dangling references (Contacts pointing to deleted Users) when the Contact is reconciled.
type UserContactController struct {
	Client client.Client

	// ContactNamespace is the namespace where new Contacts are created.
	ContactNamespace string
}

// +kubebuilder:rbac:groups=iam.miloapis.com,resources=users,verbs=get;list;watch
// +kubebuilder:rbac:groups=notification.miloapis.com,resources=contacts,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=notification.miloapis.com,resources=contacts/status,verbs=update

// Reconcile is the main reconciliation loop for the UserContactController.
func (r *UserContactController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("user-contact-controller").WithValues("user", req.Name)
	log.Info("Starting reconciliation")

	user := &iamv1alpha1.User{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: req.Name}, user); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("User not found, probably deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get User: %w", err)
	}

	// Skip if user is being deleted - we don't use finalizers, so just return
	if !user.DeletionTimestamp.IsZero() {
		log.Info("User is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Ensure a Contact exists for this User
	if err := r.ensureContactForUser(ctx, user); err != nil {
		log.Error(err, "Failed to ensure contact for user")
		return ctrl.Result{}, err
	}

	log.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

// ReconcileContact is called when a Contact is created/updated/deleted.
// It checks if the Contact's SubjectRef points to a valid User.
// If the User no longer exists, it removes the SubjectRef (cleanup of dangling reference).
func (r *UserContactController) ReconcileContact(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("user-contact-controller").WithValues("contact", req.NamespacedName)

	contact := &notificationv1alpha1.Contact{}
	if err := r.Client.Get(ctx, req.NamespacedName, contact); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get Contact: %w", err)
	}

	// Only process contacts that have a SubjectRef pointing to a User
	if contact.Spec.SubjectRef == nil || contact.Spec.SubjectRef.Kind != "User" {
		return ctrl.Result{}, nil
	}

	// Check if the referenced User exists
	user := &iamv1alpha1.User{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: contact.Spec.SubjectRef.Name}, user)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// User no longer exists, remove the SubjectRef
			log.Info("Referenced user no longer exists, removing SubjectRef", "user", contact.Spec.SubjectRef.Name)
			return r.removeSubjectRefFromContact(ctx, contact)
		}
		return ctrl.Result{}, fmt.Errorf("failed to get User: %w", err)
	}

	// Also treat a user being deleted as "no longer exists"
	if !user.DeletionTimestamp.IsZero() {
		log.Info("Referenced user is being deleted, removing SubjectRef", "user", contact.Spec.SubjectRef.Name)
		return r.removeSubjectRefFromContact(ctx, contact)
	}

	// User exists and is not being deleted - no cleanup needed
	return ctrl.Result{}, nil
}

// removeSubjectRefFromContact removes the SubjectRef from a Contact.
func (r *UserContactController) removeSubjectRefFromContact(ctx context.Context, contact *notificationv1alpha1.Contact) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("remove-subject-ref")

	userName := ""
	if contact.Spec.SubjectRef != nil {
		userName = contact.Spec.SubjectRef.Name
	}

	before := contact.DeepCopy()
	contact.Spec.SubjectRef = nil

	if err := r.Client.Patch(ctx, contact, client.MergeFrom(before), client.FieldOwner(userContactFieldOwner)); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to remove SubjectRef from contact: %w", err)
	}

	// Update the status with UserSynced condition set to False
	meta.SetStatusCondition(&contact.Status.Conditions, metav1.Condition{
		Type:               ContactUserSyncedCondition,
		Status:             metav1.ConditionFalse,
		Reason:             ContactUserUnlinkedReason,
		Message:            fmt.Sprintf("User reference removed (user %s no longer exists)", userName),
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Client.Status().Update(ctx, contact); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update contact status: %w", err)
	}

	log.Info("Removed dangling SubjectRef from contact", "contact", contact.Name, "user", userName)
	return ctrl.Result{}, nil
}

// ensureContactForUser ensures that a Contact exists for the given User and keeps it in sync.
// It first searches for Contacts that already reference this User (by SubjectRef.Name).
// If found, it syncs the email address if it has changed.
// If not found, it searches by email or creates a new Contact.
func (r *UserContactController) ensureContactForUser(ctx context.Context, user *iamv1alpha1.User) error {
	log := log.FromContext(ctx).WithName("ensure-contact-for-user").WithValues("user", user.Name, "email", user.Spec.Email)

	// First, search for Contacts that already reference this user (by SubjectRef.Name).
	// This handles the case where the user's email has changed.
	existingContactList := &notificationv1alpha1.ContactList{}
	if err := r.Client.List(ctx, existingContactList,
		client.MatchingFields{contactSubjectNameIndexKey: user.Name}); err != nil {
		return fmt.Errorf("failed to list contacts by subject name: %w", err)
	}

	// Filter to only contacts that reference this User (Kind=User check)
	for i := range existingContactList.Items {
		contact := &existingContactList.Items[i]
		if contact.Spec.SubjectRef != nil &&
			contact.Spec.SubjectRef.Kind == "User" &&
			contact.Spec.SubjectRef.Name == user.Name {
			log.Info("Found existing contact referencing this user", "contact", contact.Name)
			return r.syncContactWithUser(ctx, contact, user)
		}
	}

	// No contact referencing this user found.
	// Search for existing Contacts with the same email across all namespaces.
	contactList := &notificationv1alpha1.ContactList{}
	if err := r.Client.List(ctx, contactList,
		client.MatchingFields{contactEmailIndexKey: user.Spec.Email}); err != nil {
		return fmt.Errorf("failed to list contacts by email: %w", err)
	}

	if len(contactList.Items) > 0 {
		// Contact with same email exists, update it with SubjectRef
		contact := &contactList.Items[0]
		log.Info("Found existing contact with same email", "contact", contact.Name)
		return r.syncContactWithUser(ctx, contact, user)
	}

	// No Contact found, create a new one
	log.Info("No contact found, creating new contact")
	return r.createContactForUser(ctx, user)
}

// syncContactWithUser synchronizes the Contact with the User's data.
// It updates the SubjectRef to point to the User and syncs email.
func (r *UserContactController) syncContactWithUser(ctx context.Context, contact *notificationv1alpha1.Contact, user *iamv1alpha1.User) error {
	log := log.FromContext(ctx).WithName("sync-contact-with-user")

	// Check if any updates are needed
	needsUpdate := false

	// Check SubjectRef
	if contact.Spec.SubjectRef == nil ||
		contact.Spec.SubjectRef.Kind != "User" ||
		contact.Spec.SubjectRef.Name != user.Name {
		needsUpdate = true
	}

	// Check email
	if contact.Spec.Email != user.Spec.Email {
		needsUpdate = true
	}

	// Check given name
	if contact.Spec.GivenName != user.Spec.GivenName {
		needsUpdate = true
	}

	// Check family name
	if contact.Spec.FamilyName != user.Spec.FamilyName {
		needsUpdate = true
	}

	if !needsUpdate {
		log.Info("Contact already in sync with user, no update needed", "contact", contact.Name)
		return nil
	}

	// Update the Contact
	before := contact.DeepCopy()
	contact.Spec.SubjectRef = &notificationv1alpha1.SubjectReference{
		APIGroup: iamv1alpha1.SchemeGroupVersion.Group,
		Kind:     "User",
		Name:     user.Name,
		// Namespace is omitted for cluster-scoped resources like User
	}
	contact.Spec.Email = user.Spec.Email
	contact.Spec.GivenName = user.Spec.GivenName
	contact.Spec.FamilyName = user.Spec.FamilyName

	if err := r.Client.Patch(ctx, contact, client.MergeFrom(before), client.FieldOwner(userContactFieldOwner)); err != nil {
		return fmt.Errorf("failed to sync contact with user: %w", err)
	}

	// Update the status with UserSynced condition
	meta.SetStatusCondition(&contact.Status.Conditions, metav1.Condition{
		Type:               ContactUserSyncedCondition,
		Status:             metav1.ConditionTrue,
		Reason:             ContactUserSyncedReason,
		Message:            fmt.Sprintf("Contact synced with user %s", user.Name),
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Client.Status().Update(ctx, contact); err != nil {
		return fmt.Errorf("failed to update contact status: %w", err)
	}

	log.Info("Synced contact with user", "contact", contact.Name, "user", user.Name, "email", user.Spec.Email)
	return nil
}

// createContactForUser creates a new Contact for the given User.
func (r *UserContactController) createContactForUser(ctx context.Context, user *iamv1alpha1.User) error {
	log := log.FromContext(ctx).WithName("create-contact-for-user")

	contactName := fmt.Sprintf("user-%s", user.Name)

	contact := &notificationv1alpha1.Contact{
		ObjectMeta: metav1.ObjectMeta{
			Name:      contactName,
			Namespace: r.ContactNamespace,
		},
		Spec: notificationv1alpha1.ContactSpec{
			Email:      user.Spec.Email,
			GivenName:  user.Spec.GivenName,
			FamilyName: user.Spec.FamilyName,
			SubjectRef: &notificationv1alpha1.SubjectReference{
				APIGroup: iamv1alpha1.SchemeGroupVersion.Group,
				Kind:     "User",
				Name:     user.Name,
				// Namespace is omitted for cluster-scoped resources like User
			},
		},
	}

	if err := r.Client.Create(ctx, contact); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// Contact already exists (race condition), try to update it instead
			log.Info("Contact already exists, attempting to update", "contact", contactName)
			existingContact := &notificationv1alpha1.Contact{}
			if getErr := r.Client.Get(ctx, types.NamespacedName{Name: contactName, Namespace: r.ContactNamespace}, existingContact); getErr != nil {
				return fmt.Errorf("failed to get existing contact: %w", getErr)
			}
			return r.syncContactWithUser(ctx, existingContact, user)
		}
		return fmt.Errorf("failed to create contact: %w", err)
	}

	// Update the status with UserSynced condition
	meta.SetStatusCondition(&contact.Status.Conditions, metav1.Condition{
		Type:               ContactUserSyncedCondition,
		Status:             metav1.ConditionTrue,
		Reason:             ContactUserSyncedReason,
		Message:            fmt.Sprintf("Contact created and synced with user %s", user.Name),
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Client.Status().Update(ctx, contact); err != nil {
		return fmt.Errorf("failed to update contact status: %w", err)
	}

	log.Info("Created contact for user", "contact", contactName, "user", user.Name)
	return nil
}

// findContactsForDeletedUser finds all Contacts that reference a given User.
// Used to trigger Contact reconciliation when a User is deleted.
func (r *UserContactController) findContactsForDeletedUser(ctx context.Context, obj client.Object) []reconcile.Request {
	user, ok := obj.(*iamv1alpha1.User)
	if !ok {
		return nil
	}

	// Only trigger for deleted users
	if user.DeletionTimestamp.IsZero() {
		return nil
	}

	log := log.FromContext(ctx).WithName("find-contacts-for-deleted-user")

	// Find all contacts that reference this user
	contactList := &notificationv1alpha1.ContactList{}
	if err := r.Client.List(ctx, contactList,
		client.MatchingFields{contactSubjectNameIndexKey: user.Name}); err != nil {
		log.Error(err, "Failed to list contacts for deleted user")
		return nil
	}

	var requests []reconcile.Request
	for _, contact := range contactList.Items {
		if contact.Spec.SubjectRef != nil &&
			contact.Spec.SubjectRef.Kind == "User" &&
			contact.Spec.SubjectRef.Name == user.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      contact.Name,
					Namespace: contact.Namespace,
				},
			})
		}
	}

	if len(requests) > 0 {
		log.Info("Found contacts to clean up for deleted user", "user", user.Name, "count", len(requests))
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserContactController) SetupWithManager(mgr ctrl.Manager) error {
	// Create an index on Contact.spec.email for efficient lookups when creating contacts
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &notificationv1alpha1.Contact{}, contactEmailIndexKey, func(obj client.Object) []string {
		contact, ok := obj.(*notificationv1alpha1.Contact)
		if !ok {
			return nil
		}
		if contact.Spec.Email == "" {
			return nil
		}
		return []string{contact.Spec.Email}
	}); err != nil {
		return fmt.Errorf("failed to set field index on Contact by spec.email: %w", err)
	}

	// Create an index on Contact.spec.subject.name for efficient lookups
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &notificationv1alpha1.Contact{}, contactSubjectNameIndexKey, func(obj client.Object) []string {
		contact, ok := obj.(*notificationv1alpha1.Contact)
		if !ok {
			return nil
		}
		if contact.Spec.SubjectRef == nil || contact.Spec.SubjectRef.Name == "" {
			return nil
		}
		return []string{contact.Spec.SubjectRef.Name}
	}); err != nil {
		return fmt.Errorf("failed to set field index on Contact by spec.subject.name: %w", err)
	}

	// Build the controller that watches Users
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&iamv1alpha1.User{}).
		Named("user-contact").
		Complete(r); err != nil {
		return fmt.Errorf("failed to build user-contact controller: %w", err)
	}

	// Build a separate controller that watches Contacts for cleanup
	// This handles removing dangling SubjectRefs when Users are deleted
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&notificationv1alpha1.Contact{}).
		Watches(&iamv1alpha1.User{}, handler.EnqueueRequestsFromMapFunc(r.findContactsForDeletedUser)).
		Named("user-contact-cleanup").
		Complete(reconcile.Func(r.ReconcileContact)); err != nil {
		return fmt.Errorf("failed to build user-contact-cleanup controller: %w", err)
	}

	return nil
}
