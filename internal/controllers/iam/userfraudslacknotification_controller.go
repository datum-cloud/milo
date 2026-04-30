package iam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// TODO: remove entire controller once ArgoCD sensor is deployed.

const (
	fraudRiskSlackNotifiedAnnotation = "iam.miloapis.com/fraud-risk-slack-notified"

	fraudEvaluationGroup   = "fraud.miloapis.com"
	fraudEvaluationVersion = "v1alpha1"
	fraudEvaluationKind    = "FraudEvaluation"

	fraudPhaseCompleted   = "Completed"
	fraudDecisionAccepted = "ACCEPTED"
)

// UserFraudSlackNotificationController posts a Slack notification when a
// FraudEvaluation completes with a non-ACCEPTED decision (REVIEW or
// DEACTIVATE), i.e. the user was not auto-approved due to their fraud score.
type UserFraudSlackNotificationController struct {
	Client          client.Client
	SlackWebhookURL string

	httpClient *http.Client
}

func fraudEvaluationGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   fraudEvaluationGroup,
		Version: fraudEvaluationVersion,
		Kind:    fraudEvaluationKind,
	}
}

func newFraudEvaluation() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(fraudEvaluationGVK())
	return u
}

// shouldNotify returns true when the FraudEvaluation status indicates a
// completed evaluation with a decision other than ACCEPTED.
func shouldNotify(fe *unstructured.Unstructured) bool {
	phase, _, _ := unstructured.NestedString(fe.Object, "status", "phase")
	if phase != fraudPhaseCompleted {
		return false
	}
	decision, _, _ := unstructured.NestedString(fe.Object, "status", "decision")
	if decision == "" || decision == fraudDecisionAccepted {
		return false
	}
	return true
}

// +kubebuilder:rbac:groups=fraud.miloapis.com,resources=fraudevaluations,verbs=get;list;watch;patch;update

// Reconcile sends a Slack notification the first time a FraudEvaluation lands
// in a non-ACCEPTED terminal state.
func (r *UserFraudSlackNotificationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("user-fraud-slack-notification-controller").WithValues("fraudEvaluation", req.Name)

	fe := newFraudEvaluation()
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: req.Name}, fe); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("FraudEvaluation not found, probably deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get FraudEvaluation")
		return ctrl.Result{}, fmt.Errorf("failed to get FraudEvaluation %q: %w", req.Name, err)
	}

	if !fe.GetDeletionTimestamp().IsZero() {
		logger.Info("FraudEvaluation is being deleted, skipping Slack notification")
		return ctrl.Result{}, nil
	}

	if fe.GetAnnotations()[fraudRiskSlackNotifiedAnnotation] == "true" {
		logger.V(1).Info("Slack notification already sent, skipping")
		return ctrl.Result{}, nil
	}

	if !shouldNotify(fe) {
		logger.V(1).Info("FraudEvaluation does not require notification yet")
		return ctrl.Result{}, nil
	}

	userName, _, _ := unstructured.NestedString(fe.Object, "spec", "userRef", "name")
	if userName == "" {
		logger.Info("FraudEvaluation has no spec.userRef.name, skipping")
		return ctrl.Result{}, nil
	}

	user := &iamv1alpha1.User{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: userName}, user); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Referenced User not found, skipping", "user", userName)
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get referenced User", "user", userName)
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	decision, _, _ := unstructured.NestedString(fe.Object, "status", "decision")
	score, _, _ := unstructured.NestedString(fe.Object, "status", "compositeScore")

	if err := r.sendSlackNotification(ctx, user, decision, score); err != nil {
		logger.Error(err, "failed to send Slack notification", "user", user.Name)
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latest := newFraudEvaluation()
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: fe.GetNamespace(), Name: fe.GetName()}, latest); err != nil {
			return err
		}

		original := latest.DeepCopy()
		annotations := latest.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[fraudRiskSlackNotifiedAnnotation] = "true"
		latest.SetAnnotations(annotations)

		return r.Client.Patch(ctx, latest, client.MergeFrom(original))
	}); err != nil {
		logger.Error(err, "failed to patch FraudEvaluation with notification annotation after retries")
	}

	logger.Info("Slack notification sent for fraud-held user", "user", user.Name, "decision", decision, "compositeScore", score)
	return ctrl.Result{}, nil
}

func (r *UserFraudSlackNotificationController) sendSlackNotification(ctx context.Context, user *iamv1alpha1.User, decision, score string) error {
	logger := log.FromContext(ctx).WithName("user-fraud-slack-notification-controller")

	displayName := user.Spec.GivenName
	if displayName != "" && user.Spec.FamilyName != "" {
		displayName = fmt.Sprintf("%s %s", user.Spec.GivenName, user.Spec.FamilyName)
	} else if displayName == "" && user.Spec.FamilyName != "" {
		displayName = user.Spec.FamilyName
	}
	if displayName == "" {
		displayName = user.Spec.Email
	}

	link := fmt.Sprintf("https://staff.datum.net/customers/users/%s", user.Name)

	detail := fmt.Sprintf("Customer: <%s|%s> - %s\nDecision: *%s*", link, displayName, user.Spec.Email, decision)
	if score != "" {
		detail = fmt.Sprintf("%s\nMaxMind score: *%s*", detail, score)
	}

	payload := map[string]interface{}{
		"blocks": []map[string]interface{}{
			{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": "*User held for review (fraud score)*",
				},
			},
		},
		"attachments": []map[string]interface{}{
			{
				"color": "#e01e5a",
				"blocks": []map[string]interface{}{
					{
						"type": "section",
						"text": map[string]interface{}{
							"type": "mrkdwn",
							"text": detail,
						},
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.SlackWebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create Slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Slack webhook: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			logger.Error(cerr, "failed to close Slack response body")
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook returned non-success status: %s", resp.Status)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserFraudSlackNotificationController) SetupWithManager(mgr ctrl.Manager) error {
	r.httpClient = &http.Client{Timeout: 10 * time.Second}

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				if e.Object == nil {
					return false
				}
				if e.Object.GetAnnotations()[fraudRiskSlackNotifiedAnnotation] == "true" {
					return false
				}
				u, ok := e.Object.(*unstructured.Unstructured)
				if !ok {
					return false
				}
				return shouldNotify(u)
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				if e.ObjectNew == nil {
					return false
				}
				if e.ObjectNew.GetAnnotations()[fraudRiskSlackNotifiedAnnotation] == "true" {
					return false
				}
				newU, ok := e.ObjectNew.(*unstructured.Unstructured)
				if !ok {
					return false
				}
				if !shouldNotify(newU) {
					return false
				}
				oldU, ok := e.ObjectOld.(*unstructured.Unstructured)
				if !ok {
					return true
				}
				// Only enqueue when the qualifying state is new — avoids storms
				// on history appends or other unrelated status updates.
				return !shouldNotify(oldU)
			},
			DeleteFunc:  func(e event.DeleteEvent) bool { return false },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		}).
		For(newFraudEvaluation()).
		Named("user-fraud-slack-notification").
		Complete(r)
}
