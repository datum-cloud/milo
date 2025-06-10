package resourcemanager

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"go.miloapis.com/milo/internal/crossclusterutil"
	resourcemanagerv1alpha "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
)

const projectFinalizer = "resourcemanager.datumapis.com/project-controller"

var namespaceNotDeletedErr = errors.New("namespace has not been fully deleted")

// ProjectController reconciles a Project object
type ProjectController struct {
	ControlPlaneClient client.Client
	InfraClient        client.Client

	finalizers finalizer.Finalizers
}

//+kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=projects,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=projects/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=projects/finalizers,verbs=update

func (r *ProjectController) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	logger := log.FromContext(ctx)

	var project resourcemanagerv1alpha.Project
	if err := r.ControlPlaneClient.Get(ctx, req.NamespacedName, &project); apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get project: %w", err)
	}

	finalizationResult, err := r.finalizers.Finalize(ctx, &project)
	if err != nil {
		if v, ok := err.(kerrors.Aggregate); ok && v.Is(namespaceNotDeletedErr) {
			logger.Info(err.Error())
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to finalize: %w", err)
	}

	if finalizationResult.Updated {
		if err = r.ControlPlaneClient.Update(ctx, &project); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update based on finalization result: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// Don't need to continue if the project is being deleted from the cluster.
	if !project.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	readyCondition := apimeta.FindStatusCondition(project.Status.Conditions, resourcemanagerv1alpha.ProjectReady)
	if readyCondition == nil {
		readyCondition = &metav1.Condition{
			Type:               resourcemanagerv1alpha.ProjectReady,
			Status:             metav1.ConditionFalse,
			Reason:             "Unknown",
			ObservedGeneration: project.Generation,
		}
	} else {
		readyCondition = readyCondition.DeepCopy()
	}

	if readyCondition.Status == metav1.ConditionTrue {
		// We don't need to reconcile anything if the project is already in a ready
		// state.
		return ctrl.Result{}, nil
	}

	logger.Info("reconciling project")
	defer logger.Info("reconcile complete")

	// Create a ProjectControlPlane in the infra control plane for this project if
	// one does not exist. The name will be identical to the Project's name.
	var projectControlPlane resourcemanagerv1alpha.ProjectControlPlane
	if err := r.InfraClient.Get(ctx, req.NamespacedName, &projectControlPlane); client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}

	if projectControlPlane.CreationTimestamp.IsZero() {
		logger.Info("creating project control plane")
		projectControlPlane = resourcemanagerv1alpha.ProjectControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: project.Namespace,
				Name:      project.Name,
				Labels: map[string]string{
					crossclusterutil.ProjectNameLabel: project.Name,
				},
				Annotations: map[string]string{
					crossclusterutil.OwnerNameLabel: project.Spec.OwnerRef.Name,
				},
			},
			Spec: resourcemanagerv1alpha.ProjectControlPlaneSpec{},
		}

		if err := r.InfraClient.Create(ctx, &projectControlPlane); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create project control plane: %w", err)
		}
	}

	projectControlPlaneReadyCondition := apimeta.FindStatusCondition(projectControlPlane.Status.Conditions, resourcemanagerv1alpha.ProjectControlPlaneReady)

	if projectControlPlaneReadyCondition == nil || projectControlPlaneReadyCondition.Status == metav1.ConditionFalse {
		logger.Info("project control plane is not ready")

		if projectControlPlaneReadyCondition == nil {
			readyCondition.Reason = resourcemanagerv1alpha.ProjectProvisioningReason
			readyCondition.Message = "Project is provisioning"
		} else {
			readyCondition.Reason = projectControlPlaneReadyCondition.Reason
			readyCondition.Message = projectControlPlaneReadyCondition.Message
		}
		if apimeta.SetStatusCondition(&project.Status.Conditions, *readyCondition) {
			if err := r.ControlPlaneClient.Status().Update(ctx, &project); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed updating project status; %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	logger.Info("project control plane is ready")

	readyCondition.Status = metav1.ConditionTrue
	readyCondition.Reason = resourcemanagerv1alpha.ProjectReady
	readyCondition.Message = "Project is ready"

	if apimeta.SetStatusCondition(&project.Status.Conditions, *readyCondition) {
		if err := r.ControlPlaneClient.Status().Update(ctx, &project); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating project status; %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *ProjectController) Finalize(
	ctx context.Context,
	obj client.Object,
) (finalizer.Result, error) {
	project := obj.(*resourcemanagerv1alpha.Project)

	var projectControlPlane resourcemanagerv1alpha.ProjectControlPlane
	projectControlPlaneObjectKey := client.ObjectKeyFromObject(project)
	if err := r.InfraClient.Get(ctx, projectControlPlaneObjectKey, &projectControlPlane); client.IgnoreNotFound(err) != nil {
		return finalizer.Result{}, fmt.Errorf("failed fetching project control plane: %w", err)
	}

	if !projectControlPlane.CreationTimestamp.IsZero() && projectControlPlane.DeletionTimestamp.IsZero() {
		if err := r.InfraClient.Delete(ctx, &projectControlPlane); err != nil {
			return finalizer.Result{}, fmt.Errorf("failed deleting project control plane: %w", err)
		}
	}

	return finalizer.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProjectController) SetupWithManager(mgr ctrl.Manager, infraCluster cluster.Cluster) error {
	r.InfraClient = infraCluster.GetClient()
	r.ControlPlaneClient = mgr.GetClient()

	r.finalizers = finalizer.NewFinalizers()
	if err := r.finalizers.Register(projectFinalizer, r); err != nil {
		return fmt.Errorf("failed to register finalizer: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&resourcemanagerv1alpha.Project{}).
		WatchesRawSource(source.TypedKind(
			infraCluster.GetCache(),
			&resourcemanagerv1alpha.ProjectControlPlane{},
			handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, projectControlPlane *resourcemanagerv1alpha.ProjectControlPlane) []ctrl.Request {
				projectName, ok := projectControlPlane.Labels[crossclusterutil.ProjectNameLabel]
				if !ok {
					return nil
				}
				return []ctrl.Request{
					{
						NamespacedName: types.NamespacedName{
							Name: projectName,
						},
					},
				}
			}),
		)).
		Named("project").
		Complete(r)
}
