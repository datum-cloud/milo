package documents

import (
	"context"
	"fmt"

	documentationv1alpha1 "go.miloapis.com/milo/pkg/apis/documentation/v1alpha1"
	"go.miloapis.com/milo/pkg/util/hash"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// DocumentRevisionController reconciles a DocumentRevision object
type DocumentRevisionController struct {
	Client client.Client
}

// +kubebuilder:rbac:groups=documentation.miloapis.com,resources=documentrevisions,verbs=get;list;watch
// +kubebuilder:rbac:groups=documentation.miloapis.com,resources=documentrevisions/status,verbs=update;patch

func (r *DocumentRevisionController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithValues("controller", "DocumentRevisionController", "trigger", req.NamespacedName)
	log.Info("Starting reconciliation", "namespacedName", req.String(), "name", req.Name, "namespace", req.Namespace)

	// Get document revision
	var documentRevision documentationv1alpha1.DocumentRevision
	if err := r.Client.Get(ctx, req.NamespacedName, &documentRevision); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Document revision not found. Probably deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get document revision")
		return ctrl.Result{}, err
	}

	// DocumentRevision does not allows updates to its status, as the hash is used
	// to detect if the content of the document revision has changed.
	if meta.IsStatusConditionTrue(documentRevision.Status.Conditions, "Ready") {
		log.Info("Document revision already reconciled")
		return ctrl.Result{}, nil
	}

	documentRevision.Status.ContentHash = hash.SHA256Hex(documentRevision.Spec.Content.Data)

	// Update document revision status
	meta.SetStatusCondition(&documentRevision.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "Document revision reconciled",
		ObservedGeneration: documentRevision.Generation,
	})
	if err := r.Client.Status().Update(ctx, &documentRevision); err != nil {
		log.Error(err, "Failed to update document revision status")
		return ctrl.Result{}, fmt.Errorf("failed to update document revision status: %w", err)
	}

	log.Info("Document revision reconciled")

	return ctrl.Result{}, nil
}

func (r *DocumentRevisionController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&documentationv1alpha1.DocumentRevision{}).
		Named("documentrevision").
		Complete(r)
}
