package resourcemanager

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
func (r *ProjectController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var project resourcemanagerv1alpha.Project
	if err := r.ControlPlaneClient.Get(ctx, req.NamespacedName, &project); apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, fmt.Errorf("get project: %w", err)
	}

	// Deletion path: run purge, then remove finalizer
	if !project.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&project, projectFinalizer) {
			projCfg := r.forProject(r.BaseConfig, project.Name)
			if err := r.Purger.Purge(ctx, projCfg, project.Name, projectpurge.Options{
				Timeout:  10 * time.Minute,
				Parallel: 16,
			}); err != nil {
				// requeue to retry purge
				return ctrl.Result{RequeueAfter: 2 * time.Second}, fmt.Errorf("purge %q: %w", project.Name, err)
			}
			before := project.DeepCopy()
			controllerutil.RemoveFinalizer(&project, projectFinalizer)
			if err := r.ControlPlaneClient.Patch(ctx, &project, client.MergeFrom(before)); err != nil {
				return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer present
	if !controllerutil.ContainsFinalizer(&project, projectFinalizer) {
		before := project.DeepCopy()
		controllerutil.AddFinalizer(&project, projectFinalizer)
		if err := r.ControlPlaneClient.Patch(ctx, &project, client.MergeFrom(before)); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		// trigger another reconcile after patch
		return ctrl.Result{}, nil
	}

	// Ensure per-project "default" Namespace exists
	projCfg := r.forProject(r.BaseConfig, project.Name)
	if err := ensureDefaultNamespace(ctx, projCfg); err != nil {
		logger.Error(err, "ensure default namespace failed", "project", project.Name)
		// Backoff and retry; don't mark Ready yet
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	// Set Ready condition (idempotent)
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

func ensureDefaultNamespace(ctx context.Context, cfg *rest.Config) error {
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("build project client: %w", err)
	}

	// Quick GET first (cheap, idempotent)
	if _, err := cs.CoreV1().Namespaces().Get(ctx, metav1.NamespaceDefault, metav1.GetOptions{}); err == nil {
		return nil
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get namespace %q: %w", metav1.NamespaceDefault, err)
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: metav1.NamespaceDefault,
			Labels: map[string]string{
				"miloapis.com/project-default": "true",
			},
		},
	}
	if _, err := cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create namespace %q: %w", ns.Name, err)
	}
	return nil
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
