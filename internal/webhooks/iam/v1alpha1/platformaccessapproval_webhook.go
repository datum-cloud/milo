package v1alpha1

import (
	"context"
	"fmt"
	"net/mail"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
)

func SetupPlatformAccessApprovalWebhooksWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&iamv1alpha1.PlatformAccessApproval{}).
		WithDefaulter(&PlatformAccessApprovalMutator{
			client: mgr.GetClient(),
		}).
		WithValidator(&PlatformAccessApprovalValidator{
			client: mgr.GetClient(),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-iam-miloapis-com-v1alpha1-platformaccessapproval,mutating=true,failurePolicy=fail,sideEffects=None,groups=iam.miloapis.com,resources=platformaccessapprovals,verbs=create,versions=v1alpha1,name=mplatformaccessapproval.iam.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

// PlatformAccessApprovalMutator mutates PlatformAccessApproval resources to set the approver to the user who is approving the access request.
type PlatformAccessApprovalMutator struct {
	client client.Client
}

func (m *PlatformAccessApprovalMutator) Default(ctx context.Context, obj runtime.Object) error {
	paa, ok := obj.(*iamv1alpha1.PlatformAccessApproval)
	if !ok {
		return errors.NewInternalError(fmt.Errorf("failed to cast object to PlatformAccessApproval"))
	}
	log := logf.FromContext(ctx).WithValues("Defaulting PlatformAccessApproval", "name", paa.GetName())

	// Approver is the user who is approving the access request.
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		log.Error(err, "failed to get admission request from context", "name", paa.GetName())
		return errors.NewInternalError(fmt.Errorf("failed to get request from context: %w", err))
	}

	// Get the approver user
	approverUser := &iamv1alpha1.User{}
	if err := m.client.Get(ctx, client.ObjectKey{Name: string(req.UserInfo.UID)}, approverUser); err != nil {
		// Approver should be found
		log.Error(err, "failed to get user '%s' from iam.miloapis.com API", string(req.UserInfo.UID))
		return errors.NewInternalError(fmt.Errorf("failed to get user '%s' from iam.miloapis.com API: %w", string(req.UserInfo.UID), err))
	}

	// Set the approver user
	paa.Spec.ApproverRef = &iamv1alpha1.UserReference{
		Name: approverUser.Name,
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-iam-miloapis-com-v1alpha1-platformaccessapproval,mutating=false,failurePolicy=fail,sideEffects=None,groups=iam.miloapis.com,resources=platformaccessapprovals,verbs=create,versions=v1alpha1,name=vplatformaccessapproval.iam.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

// PlatformAccessApprovalValidator validates PlatformAccessApproval resources.
type PlatformAccessApprovalValidator struct {
	client client.Client
}

func (v *PlatformAccessApprovalValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	paa := obj.(*iamv1alpha1.PlatformAccessApproval)
	log := logf.FromContext(ctx).WithValues("Validating PlatformAccessApproval", "name", paa.GetName())

	var errs field.ErrorList

	// Exactly one of subjectRef.email or subjectRef.userRef is
	// validated at API level, so we don't need to validate it here.

	// Validate subjectRef.email is valid
	emailAddress := paa.Spec.SubjectRef.Email
	if emailAddress != "" {
		if _, err := mail.ParseAddress(emailAddress); err != nil {
			log.Info("invalid email address", "email", emailAddress)
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("subjectRef").Child("email"), emailAddress, fmt.Sprintf("invalid email address: %v", err)))
		}
	}

	// Validate subjectRef.userRef is valid
	userRef := paa.Spec.SubjectRef.UserRef
	if userRef != nil {
		user := &iamv1alpha1.User{}
		if err := v.client.Get(ctx, client.ObjectKey{Name: userRef.Name}, user); err != nil {
			if errors.IsNotFound(err) {
				log.Info("user not found", "name", userRef.Name)
				errs = append(errs, field.NotFound(field.NewPath("spec").Child("subjectRef").Child("userRef").Child("name"), userRef.Name))
			} else {
				log.Error(err, "failed to get user", "name", userRef.Name)
				errs = append(errs, field.InternalError(field.NewPath("spec").Child("subjectRef").Child("userRef").Child("name"), fmt.Errorf("failed to get user: %w", err)))
			}
		}
	}

	if len(errs) > 0 {
		return nil, errors.NewInvalid(iamv1alpha1.SchemeGroupVersion.WithKind("PlatformAccessApproval").GroupKind(), paa.Name, errs)
	}

	return nil, nil
}

func (v *PlatformAccessApprovalValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *PlatformAccessApprovalValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
