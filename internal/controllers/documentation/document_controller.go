package documents

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	docversion "go.miloapis.com/milo/pkg/version"

	documentationv1alpha1 "go.miloapis.com/milo/pkg/apis/documentation/v1alpha1"
)

const (
	documentRefNamespacedKey = "documentation.miloapis.com/documentnamespacedkey"
)

func buildDocumentRevisionByDocumentIndexKey(docRef documentationv1alpha1.DocumentReference) string {
	return fmt.Sprintf("%s|%s", docRef.Name, docRef.Namespace)
}

// DocumentController reconciles a Document object
type DocumentController struct {
	Client     client.Client
	Finalizers finalizer.Finalizers
}

// +kubebuilder:rbac:groups=documentation.miloapis.com,resources=documents,verbs=get;list;watch
// +kubebuilder:rbac:groups=documentation.miloapis.com,resources=documents/status,verbs=update;patch
// +kubebuilder:rbac:groups=documentation.miloapis.com,resources=documentrevisions,verbs=get;list;watch

func (r *DocumentController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithValues("controller", "DocumentController", "trigger", req.NamespacedName)
	log.Info("Starting reconciliation", "namespacedName", req.String(), "name", req.Name, "namespace", req.Namespace)

	// Get document
	var document documentationv1alpha1.Document
	if err := r.Client.Get(ctx, req.NamespacedName, &document); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Document not found. Probably deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get document")
		return ctrl.Result{}, err
	}
	oldStatus := document.Status.DeepCopy()

	// Get document revisions
	var documentRevisions documentationv1alpha1.DocumentRevisionList
	if err := r.Client.List(ctx, &documentRevisions,
		client.MatchingFields{
			documentRefNamespacedKey: buildDocumentRevisionByDocumentIndexKey(
				documentationv1alpha1.DocumentReference{Name: document.Name, Namespace: document.Namespace})}); err != nil {
		log.Error(err, "failed to get document revisions")
		return ctrl.Result{}, err
	}
	// Verify if there is at least one revision
	revisionFound := false
	if len(documentRevisions.Items) > 0 {
		revisionFound = true
	}

	// Update document status
	meta.SetStatusCondition(&document.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "Document reconciled",
		ObservedGeneration: document.Generation,
	})
	if revisionFound {
		log.Info("Revision found. Updating latest revision status reference")
		latestRevision, err := GetLatestDocumentRevision(documentRevisions)
		if err != nil {
			log.Error(err, "failed to get latest document revision")
			return ctrl.Result{}, err
		}
		// Update latest revision status reference
		document.Status.LatestRevisionRef = &documentationv1alpha1.LatestRevisionRef{
			Name:        latestRevision.Name,
			Namespace:   latestRevision.Namespace,
			Version:     latestRevision.Spec.Version,
			PublishedAt: latestRevision.Spec.EffectiveDate,
		}
	} else {
		log.Info("No revision found. Updating latest revision status reference to nil")
		document.Status.LatestRevisionRef = nil
	}
	// Update document status onlyif it changed
	if !equality.Semantic.DeepEqual(oldStatus, &document.Status) {
		if err := r.Client.Status().Update(ctx, &document); err != nil {
			log.Error(err, "Failed to update document status")
			return ctrl.Result{}, fmt.Errorf("failed to update document status: %w", err)
		}
	} else {
		log.V(1).Info("Document status unchanged, skipping update")
	}

	log.Info("Document reconciled")

	return ctrl.Result{}, nil
}

// GetLatestDocumentRevision returns the latest document revision from the list of document revisions.
func GetLatestDocumentRevision(documentRevisions documentationv1alpha1.DocumentRevisionList) (documentationv1alpha1.DocumentRevision, error) {
	latestRevision := documentRevisions.Items[0]
	for _, revision := range documentRevisions.Items {
		isHigher, err := docversion.IsVersionHigher(revision.Spec.Version, latestRevision.Spec.Version)
		if err != nil {
			return documentationv1alpha1.DocumentRevision{}, err
		}
		if isHigher {
			latestRevision = revision
		}
	}

	return latestRevision, nil
}

// enqueueDocumentForDocumentRevisionCreate enqueues the referenced document for the document revision create event.
// This is used to ensure that the document status is updated when a document revision is created.
func (r *DocumentController) enqueueDocumentForDocumentRevisionCreate(ctx context.Context, obj client.Object) []ctrl.Request {
	log := logf.FromContext(ctx).WithValues("controller", "DocumentController", "trigger", obj.GetName())
	log.Info("Enqueuing document for document revision create")
	dr, ok := obj.(*documentationv1alpha1.DocumentRevision)
	if !ok {
		log.Error(fmt.Errorf("failed to cast object to DocumentRevision"), "failed to cast object to DocumentRevision")
		return nil
	}

	referencedDocument := &documentationv1alpha1.Document{}
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: dr.Spec.DocumentRef.Namespace, Name: dr.Spec.DocumentRef.Name}, referencedDocument); err != nil {
		// Document must exists, as webhook validates it
		log.Error(err, "failed to get referenced document", "namespace", dr.Spec.DocumentRef.Namespace, "name", dr.Spec.DocumentRef.Name)
		return nil
	}
	log.V(1).Info("Referenced document found. Enqueuing document", "namespace", referencedDocument.Namespace, "name", referencedDocument.Name)

	return []ctrl.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      referencedDocument.Name,
				Namespace: referencedDocument.Namespace,
			},
		},
	}
}

func (r *DocumentController) SetupWithManager(mgr ctrl.Manager) error {
	// Index DocumentRevision by documentref namespaced key for efficient lookups
	if err := mgr.GetFieldIndexer().IndexField(context.Background(),
		&documentationv1alpha1.DocumentRevision{}, documentRefNamespacedKey, func(obj client.Object) []string {
			dr, ok := obj.(*documentationv1alpha1.DocumentRevision)
			if !ok {
				return nil
			}
			return []string{buildDocumentRevisionByDocumentIndexKey(dr.Spec.DocumentRef)}
		}); err != nil {
		return fmt.Errorf("failed to set field index on DocumentRevision: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&documentationv1alpha1.Document{}).
		Watches(
			&documentationv1alpha1.DocumentRevision{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueDocumentForDocumentRevisionCreate),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc:  func(e event.CreateEvent) bool { return true },
				DeleteFunc:  func(e event.DeleteEvent) bool { return false },
				GenericFunc: func(e event.GenericEvent) bool { return false },
				UpdateFunc:  func(e event.UpdateEvent) bool { return false },
			}),
		).
		Named("document").
		Complete(r)
}
