package resourcemanager

import (
	"context"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	resourcemanagerv1alpha "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
	"go.miloapis.com/milo/pkg/controller/projectpurge"
)

const projectFinalizer = "resourcemanager.miloapis.com/project-controller"

// ProjectController reconciles a Project object
type ProjectController struct {
	ControlPlaneClient client.Client

	// Base (root) API config used to derive per-project clients.
	BaseConfig *rest.Config

	// Purger orchestrates DeleteCollection across all resources
	Purger *projectpurge.Purger
}

// +kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=projects,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=projects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=projects/finalizers,verbs=update
func (r *ProjectController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var project resourcemanagerv1alpha.Project
	if err := r.ControlPlaneClient.Get(ctx, req.NamespacedName, &project); apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, fmt.Errorf("get project: %w", err)
	}

	logger.Info("reconciling project")
	defer logger.Info("reconcile complete")

	// ----- ensure finalizer on live objects -----
	if project.DeletionTimestamp.IsZero() {
		// add finalizer if missing
		if !controllerutil.ContainsFinalizer(&project, projectFinalizer) {
			before := project.DeepCopy()
			controllerutil.AddFinalizer(&project, projectFinalizer)
			if err := r.ControlPlaneClient.Patch(ctx, &project, client.MergeFrom(before)); err != nil {
				return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
			}
			// will reconcile again after patch; nothing else to do now
			return ctrl.Result{}, nil
		}

		// (optional) set Ready condition
		if cond := apimeta.FindStatusCondition(project.Status.Conditions, resourcemanagerv1alpha.ProjectReady); cond == nil || cond.Status != metav1.ConditionTrue {
			newCond := metav1.Condition{
				Type:               resourcemanagerv1alpha.ProjectReady,
				Status:             metav1.ConditionTrue,
				Reason:             resourcemanagerv1alpha.ProjectReady,
				Message:            "Project is ready",
				ObservedGeneration: project.Generation,
			}
			if apimeta.SetStatusCondition(&project.Status.Conditions, newCond) {
				_ = r.ControlPlaneClient.Status().Update(ctx, &project)
			}
		}
		return ctrl.Result{}, nil
	}

	// ----- handle deletion -----
	if controllerutil.ContainsFinalizer(&project, projectFinalizer) {
		projCfg := r.forProject(r.BaseConfig, project.Name)

		if err := r.Purger.Purge(ctx, projCfg, project.Name, projectpurge.Options{
			Timeout:  10 * time.Minute,
			Parallel: 16,
		}); err != nil {
			// requeue so we retry purge
			return ctrl.Result{}, fmt.Errorf("purge %q: %w", project.Name, err)
		}

		// remove finalizer to allow deletion to complete
		before := project.DeepCopy()
		controllerutil.RemoveFinalizer(&project, projectFinalizer)
		if err := r.ControlPlaneClient.Patch(ctx, &project, client.MergeFrom(before)); err != nil {
			return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *ProjectController) forProject(base *rest.Config, project string) *rest.Config {
	c := rest.CopyConfig(base)
	c.Host = strings.TrimSuffix(base.Host, "/") + "/projects/" + project + "/control-plane"
	return c
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProjectController) SetupWithManager(mgr ctrl.Manager) error {
	r.ControlPlaneClient = mgr.GetClient()
	r.BaseConfig = mgr.GetConfig()
	r.Purger = projectpurge.New()

	return ctrl.NewControllerManagedBy(mgr).
		For(&resourcemanagerv1alpha.Project{}).
		Named("project").
		Complete(r)
}
