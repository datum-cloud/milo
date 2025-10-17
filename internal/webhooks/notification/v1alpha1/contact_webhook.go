package v1alpha1

import (
	"context"
	"fmt"
	"net/mail"
	"reflect"
	"slices"

	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var contactLog = logf.Log.WithName("contact-resource")
var acceptedResourceManagerKinds = []string{"Organization", "Project"}
var acceptedAPIGroups = []string{"iam.miloapis.com", "resourcemanager.miloapis.com"}

// SetupContactWebhooksWithManager sets up the webhooks for the Contact resource.
func SetupContactWebhooksWithManager(mgr ctrl.Manager) error {
	contactLog.Info("Setting up notification.miloapis.com contact webhooks")

	return ctrl.NewWebhookManagedBy(mgr).
		For(&notificationv1alpha1.Contact{}).
		WithDefaulter(&ContactMutator{
			client: mgr.GetClient(),
			scheme: mgr.GetScheme(),
		}).
		WithValidator(&ContactValidator{
			Client: mgr.GetClient(),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-notification-miloapis-com-v1alpha1-contact,mutating=true,failurePolicy=fail,sideEffects=None,groups=notification.miloapis.com,resources=contacts,verbs=create,versions=v1alpha1,name=mcontact.notification.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

// ContactMutator sets default values on Contact resources.
type ContactMutator struct {
	client client.Client
	scheme *runtime.Scheme
}

func (m *ContactMutator) Default(ctx context.Context, obj runtime.Object) error {
	contact, ok := obj.(*notificationv1alpha1.Contact)
	if !ok {
		return errors.NewInternalError(fmt.Errorf("failed to cast object to Contact"))
	}
	contactLog.Info("Defaulting Contact", "name", contact.Name)

	if contact.Spec.SubjectRef != nil {
		if contact.Spec.SubjectRef.APIGroup == "iam.miloapis.com" {
			if contact.Spec.SubjectRef.Kind == "User" {
				// Set the owner reference so the Contact is garbage collected when the User is deleted.
				user := &iamv1alpha1.User{}
				if err := m.client.Get(ctx, client.ObjectKey{Name: contact.Spec.SubjectRef.Name}, user); err != nil {
					return errors.NewInternalError(fmt.Errorf("failed to fetch referenced User while setting owner reference for contact, %w", err))
				}
				if err := controllerutil.SetOwnerReference(user, contact, m.scheme); err != nil {
					return errors.NewInternalError(fmt.Errorf("failed to set owner reference for contact: %w", err))
				}
			}
		}
	}
	return nil
}

// +kubebuilder:webhook:path=/validate-notification-miloapis-com-v1alpha1-contact,mutating=false,failurePolicy=fail,sideEffects=None,groups=notification.miloapis.com,resources=contacts,verbs=create;update,versions=v1alpha1,name=vcontact.notification.miloapis.com,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

type ContactValidator struct {
	Client client.Client
}

func (v *ContactValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	errs := field.ErrorList{}
	contact, ok := obj.(*notificationv1alpha1.Contact)
	if !ok {
		return nil, errors.NewInternalError(fmt.Errorf("failed to cast object to Contact"))
	}
	contactLog.Info("Validating Contact", "name", contact.Name)

	// Validate Email format
	if _, err := mail.ParseAddress(contact.Spec.Email); err != nil {
		errs = append(errs, field.Invalid(field.NewPath("spec", "email"), contact.Spec.Email, fmt.Sprintf("invalid email address: %v", err)))
	}

	// If no subject reference is provided, we are in presense of a Contact with no related subject
	// It could be a Newsletter Contact, for example
	if contact.Spec.SubjectRef == nil {
		return nil, contactValidationResult(errs, contact)
	}

	// If here, we have a Contact with a related subject reference

	// Validate IAM API Group & Kind
	if contact.Spec.SubjectRef.APIGroup == "iam.miloapis.com" {
		// Validate Kind
		if contact.Spec.SubjectRef.Kind != "User" {
			errs = append(errs, field.Invalid(field.NewPath("spec", "subjectRef", "kind"), contact.Spec.SubjectRef.Kind, "kind must be User for iam.miloapis.com API group"))
		}
	}
	// Validate ResourceManager API Group & Kind
	if contact.Spec.SubjectRef.APIGroup == "resourcemanager.miloapis.com" {
		// Validate Kind
		if !slices.Contains(acceptedResourceManagerKinds, contact.Spec.SubjectRef.Kind) {
			errs = append(errs, field.Invalid(field.NewPath("spec", "subjectRef", "kind"), contact.Spec.SubjectRef.Kind, "kind must be one of Organization or Project for resourcemanager.miloapis.com API group"))
		}
	}
	// Validate API group (in case we add support for other API groups in the future)
	if !slices.Contains(acceptedAPIGroups, contact.Spec.SubjectRef.APIGroup) {
		errs = append(errs, field.Invalid(field.NewPath("spec", "subjectRef", "apiGroup"), contact.Spec.SubjectRef.APIGroup, "apiGroup must be one of iam.miloapis.com or resourcemanager.miloapis.com"))
	}

	// Validate User reference
	if contact.Spec.SubjectRef.Kind == "User" {
		// Validate Namespace
		if contact.Spec.SubjectRef.Namespace != "" {
			errs = append(errs, field.Invalid(field.NewPath("spec", "subjectRef", "namespace"), contact.Spec.SubjectRef.Namespace, "namespace must be empty for User"))
		}
		// If errs here, we cannot validate if the User exists
		if err := contactValidationResult(errs, contact); err != nil {
			return nil, err
		}
		// Validate User exists
		user := &iamv1alpha1.User{}
		if err := v.Client.Get(ctx, client.ObjectKey{Name: contact.Spec.SubjectRef.Name}, user); err != nil {
			if errors.IsNotFound(err) {
				errs = append(errs, field.NotFound(field.NewPath("spec", "subjectRef", "name"), contact.Spec.SubjectRef.Name))
			} else {
				return nil, errors.NewInternalError(fmt.Errorf("failed to get user: %w", err))
			}
		}
	}

	// Validate Organization reference
	if contact.Spec.SubjectRef.Kind == "Organization" {
		if contact.Spec.SubjectRef.Namespace != fmt.Sprintf("organization-%s", contact.Spec.SubjectRef.Name) {
			errs = append(errs, field.Invalid(field.NewPath("spec", "subjectRef", "namespace"), contact.Spec.SubjectRef.Namespace, "namespace must be the organization namespace (organization-<organization-name>)"))
		}
		// Validate Organization exists
		organization := &resourcemanagerv1alpha1.Organization{}
		if err := v.Client.Get(ctx, client.ObjectKey{Name: contact.Spec.SubjectRef.Name}, organization); err != nil {
			if errors.IsNotFound(err) {
				errs = append(errs, field.NotFound(field.NewPath("spec", "subjectRef", "name"), contact.Spec.SubjectRef.Name))
			} else {
				return nil, errors.NewInternalError(fmt.Errorf("failed to get organization: %w", err))
			}
		}
	}

	// Validate Project reference
	if contact.Spec.SubjectRef.Kind == "Project" {
		// Validate Project exists
		project := &resourcemanagerv1alpha1.Project{}
		if err := v.Client.Get(ctx, client.ObjectKey{Name: contact.Spec.SubjectRef.Name}, project); err != nil {
			if errors.IsNotFound(err) {
				errs = append(errs, field.NotFound(field.NewPath("spec", "subjectRef", "name"), contact.Spec.SubjectRef.Name))
				return nil, contactValidationResult(errs, contact)
			} else {
				return nil, errors.NewInternalError(fmt.Errorf("failed to get project: %w", err))
			}
		}
		// Validate Namespace
		if contact.Spec.SubjectRef.Namespace != fmt.Sprintf("organization-%s", project.Spec.OwnerRef.Name) {
			errs = append(errs, field.Invalid(field.NewPath("spec", "subjectRef", "namespace"), contact.Spec.SubjectRef.Namespace, "namespace must be the project owner's namespace (organization-<organization-name>)"))
			return nil, contactValidationResult(errs, contact)
		}
	}

	return nil, contactValidationResult(errs, contact)
}

// contactValidationResult returns an error if there are any errors in the error list
func contactValidationResult(errs field.ErrorList, contact *notificationv1alpha1.Contact) error {
	if len(errs) > 0 {
		return errors.NewInvalid(notificationv1alpha1.SchemeGroupVersion.WithKind("Contact").GroupKind(), contact.Name, errs)
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *ContactValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *ContactValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	contactNew, okNew := newObj.(*notificationv1alpha1.Contact)
	contactOld, okOld := oldObj.(*notificationv1alpha1.Contact)
	if !okNew || !okOld {
		return nil, errors.NewInternalError(fmt.Errorf("failed to cast object(s) to Contact"))
	}
	errs := field.ErrorList{}

	// If the SubjectRef changed, reject the update.
	if !reflect.DeepEqual(contactNew.Spec.SubjectRef, contactOld.Spec.SubjectRef) {
		errs = append(errs, field.Invalid(field.NewPath("spec", "subjectRef"), contactNew.Spec.SubjectRef, "subjectRef is immutable and cannot be updated"))
	}

	// Validate Email format
	if contactNew.Spec.Email != contactOld.Spec.Email {
		if _, err := mail.ParseAddress(contactNew.Spec.Email); err != nil {
			errs = append(errs, field.Invalid(field.NewPath("spec", "email"), contactNew.Spec.Email, fmt.Sprintf("invalid email address: %v", err)))
		}
	}

	return nil, contactValidationResult(errs, contactNew)
}
