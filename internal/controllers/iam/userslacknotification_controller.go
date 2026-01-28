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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// TODO: remove entire controller once ArgoCD sensor is deployed.

const slackNotifiedAnnotation = "iam.miloapis.com/creation-slack-notified"

// UserSlackNotificationController sends a Slack notification whenever a new User is created.
type UserSlackNotificationController struct {
	Client          client.Client
	SlackWebhookURL string

	// httpClient is used to send requests to Slack. If nil, a default client is used.
	httpClient *http.Client
}

// +kubebuilder:rbac:groups=iam.miloapis.com,resources=users,verbs=get;list;watch;patch;update

// Reconcile sends a Slack notification the first time it observes a User.
func (r *UserSlackNotificationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("user-slack-notification-controller").WithValues("user", req.Name)

	user := &iamv1alpha1.User{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: req.Name}, user); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("User not found, probably deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get User")
		return ctrl.Result{}, fmt.Errorf("failed to get User %q: %w", req.Name, err)
	}

	// Skip deleted users.
	if !user.DeletionTimestamp.IsZero() {
		logger.Info("User is being deleted, skipping Slack notification")
		return ctrl.Result{}, nil
	}

	// Skip if notification already sent
	if user.Annotations != nil && user.Annotations[slackNotifiedAnnotation] == "true" {
		logger.V(1).Info("Slack notification already sent, skipping")
		return ctrl.Result{}, nil
	}

	if err := r.sendSlackNotification(ctx, user); err != nil {
		logger.Error(err, "failed to send Slack notification", "user", user.Name)
		// Requeue in case of transient Slack or network issues.
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	// Patch annotation to record notification sent, retrying on optimistic concurrency errors
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latest := &iamv1alpha1.User{}
		if err := r.Client.Get(ctx, types.NamespacedName{Name: user.Name}, latest); err != nil {
			return err
		}

		original := latest.DeepCopy()
		if latest.Annotations == nil {
			latest.Annotations = map[string]string{}
		}
		latest.Annotations[slackNotifiedAnnotation] = "true"

		return r.Client.Patch(ctx, latest, client.MergeFrom(original))
	}); err != nil {
		logger.Error(err, "failed to patch user with notification annotation after retries")
	}

	logger.Info("Slack notification sent for user", "user", user.Name)
	return ctrl.Result{}, nil
}

func (r *UserSlackNotificationController) sendSlackNotification(ctx context.Context, user *iamv1alpha1.User) error {
	logger := log.FromContext(ctx).WithName("user-slack-notification-controller")

	displayName := user.Spec.GivenName
	if displayName != "" && user.Spec.FamilyName != "" {
		displayName = fmt.Sprintf("%s %s", user.Spec.GivenName, user.Spec.FamilyName)
	} else if displayName == "" && user.Spec.FamilyName != "" {
		displayName = user.Spec.FamilyName
	}
	if displayName == "" {
		// Fallback to the email if no name components are present.
		displayName = user.Spec.Email
	}

	link := fmt.Sprintf("https://staff.datum.net/customers/users/%s", user.Name)

	payload := map[string]interface{}{
		"blocks": []map[string]interface{}{
			{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": "*User signed up*",
				},
			},
		},
		"attachments": []map[string]interface{}{
			{
				"color": "#f2c744",
				"blocks": []map[string]interface{}{
					{
						"type": "section",
						"text": map[string]interface{}{
							"type": "mrkdwn",
							"text": fmt.Sprintf("Customer: <%s|%s> - %s", link, displayName, user.Spec.Email),
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
func (r *UserSlackNotificationController) SetupWithManager(mgr ctrl.Manager) error {
	r.httpClient = &http.Client{Timeout: 10 * time.Second}

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				if e.Object == nil {
					return false
				}
				if e.Object.GetAnnotations()[slackNotifiedAnnotation] == "true" {
					return false
				}
				return true
			},
			UpdateFunc:  func(e event.UpdateEvent) bool { return false },
			DeleteFunc:  func(e event.DeleteEvent) bool { return false },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		}).
		For(&iamv1alpha1.User{}).
		Named("user-slack-notification").
		Complete(r)
}
