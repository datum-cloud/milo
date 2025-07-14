package app

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/dynamic"
	clientrest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// MachineAccountKeyREST implements a RESTStorage for MachineAccountKey resources.
type MachineAccountKeyREST struct {
	*registry.Store

	// dynClient is used to query the core API server for related resources
	dynClient dynamic.Interface
}

func NewMachineAccountKeyREST(scheme *runtime.Scheme, optsGetter generic.RESTOptionsGetter) *MachineAccountKeyREST {
	store := &registry.Store{
		NewFunc:                   func() runtime.Object { return &v1alpha1.MachineAccountKey{} },
		NewListFunc:               func() runtime.Object { return &v1alpha1.MachineAccountKeyList{} },
		DefaultQualifiedResource:  schema.GroupResource{Group: v1alpha1.SchemeGroupVersion.Group, Resource: "machineaccountkeys"},
		SingularQualifiedResource: schema.GroupResource{Group: v1alpha1.SchemeGroupVersion.Group, Resource: "machineaccountkey"},
		PredicateFunc: func(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
			return storage.SelectionPredicate{
				Label:    label,
				Field:    field,
				GetAttrs: GetAttrs,
			}
		},
		CreateStrategy: Strategy,
		UpdateStrategy: Strategy,
		DeleteStrategy: Strategy,
		TableConvertor: rest.NewDefaultTableConvertor(schema.GroupResource{Group: v1alpha1.SchemeGroupVersion.Group, Resource: "machineaccountkeys"}),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}
	// Build an in-cluster dynamic client so we can verify referenced resources.
	cfg, err := clientrest.InClusterConfig()
	if err != nil {
		cfg, err = clientcmd.BuildConfigFromFlags("", KubeconfigPath)
		if err != nil {
			panic(fmt.Errorf("failed to build kubeconfig: %w", err))
		}
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		panic(fmt.Errorf("failed to build dynamic client: %w", err))
	}

	return &MachineAccountKeyREST{Store: store, dynClient: dyn}
}

// Destroy is currently a no-op because registry.Store does not need explicit cleanup.
func (r *MachineAccountKeyREST) Destroy() {}

// TODO: write a custom create strategy that generates a private key and returns it in the response.
func (r *MachineAccountKeyREST) Create(
	ctx context.Context,
	obj runtime.Object,
	validate rest.ValidateObjectFunc,
	options *metav1.CreateOptions,
) (runtime.Object, error) {
	mak := obj.(*v1alpha1.MachineAccountKey)

	var privateKeyPEM string // will hold the private key if we end up generating one

	// 1. run the standard validation
	if err := r.CreateStrategy.Validate(ctx, obj); len(err) != 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "iam.miloapis.com", Kind: "MachineAccountKey"}, mak.Name, err)
	}

	// 2. verify that the referenced MachineAccount exists in the cluster
	gvr := schema.GroupVersionResource{
		Group:    v1alpha1.SchemeGroupVersion.Group,
		Version:  v1alpha1.SchemeGroupVersion.Version,
		Resource: "machineaccounts",
	}
	_, err := r.dynClient.Resource(gvr).Namespace(mak.Namespace).Get(ctx, mak.Spec.MachineAccountName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewNotFound(schema.GroupResource{Group: "iam.miloapis.com", Resource: "machineaccounts"}, mak.Spec.MachineAccountName)
		}
		return nil, err
	}

	// 3. validate expiration date if provided
	if mak.Spec.ExpirationDate != nil {
		if mak.Spec.ExpirationDate.Time.Before(time.Now().UTC()) {
			return nil, apierrors.NewBadRequest("spec.expirationDate must be a future timestamp")
		}
	}

	// 3. generate new key pair or validate provided public key
	if mak.Spec.PublicKey == "" {
		// Generate new RSA key pair (2048 bits)
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("failed to generate RSA key: %w", err))
		}

		// Encode private key in PKCS#1 PEM format to return in response
		privBytes := x509.MarshalPKCS1PrivateKey(key)
		privPem := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes})
		privateKeyPEM = string(privPem)

		// Encode public key in PKIX PEM format to store in the resource
		pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("failed to marshal public key: %w", err))
		}
		pubPem := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
		mak.Spec.PublicKey = string(pubPem)
	} else {
		// Verify provided public key is valid PEM and parseable
		block, _ := pem.Decode([]byte(mak.Spec.PublicKey))
		if block == nil {
			return nil, apierrors.NewBadRequest("spec.publicKey must be a valid PEM encoded key")
		}

		if _, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
			if _, err2 := x509.ParsePKCS1PublicKey(block.Bytes); err2 != nil {
				return nil, apierrors.NewBadRequest("spec.publicKey is not a valid RSA public key")
			}
		}

	}

	// 4. persist to etcd
	createdObj, err := r.Store.Create(ctx, obj, validate, options)
	if err != nil {
		return nil, err
	}

	// 5. Build the response object. We include the generated private key if we created one.
	response := &unstructured.Unstructured{}
	response.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "iam.miloapis.com",
		Version: "v1alpha1",
		Kind:    "MachineAccountKeyResponse",
	})

	response.Object["status"] = map[string]any{
		"message": "key created",
		"id":      createdObj.(*v1alpha1.MachineAccountKey).UID,
		"payload": map[string]any{
			"type":       "serviceaccount",
			"userId":     createdObj.(*v1alpha1.MachineAccountKey).Spec.MachineAccountName,
			"privateKey": privateKeyPEM, // whatever you want to return
			"expirationDate": func() string {
				if createdObj.(*v1alpha1.MachineAccountKey).Spec.ExpirationDate != nil {
					return createdObj.(*v1alpha1.MachineAccountKey).Spec.ExpirationDate.Time.UTC().Format(time.RFC3339)
				}
				return ""
			}(),
		},
	}

	return response, nil
}

// GetAttrs returns labels and fields for a MachineAccountKey object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	machineAccountKey, ok := obj.(*v1alpha1.MachineAccountKey)
	if !ok {
		return nil, nil, storage.NewInvalidObjError("expected a MachineAccountKey object", "msg")
	}

	return labels.Set(machineAccountKey.ObjectMeta.Labels), fields.Set{
		"metadata.name":      machineAccountKey.Name,
		"metadata.namespace": machineAccountKey.Namespace,
	}, nil
}

// Strategy implements the logic for create/update/delete for MachineAccountKey.
var Strategy = &machineAccountKeyStrategy{}

// TODO: review all strategies. This is a copy of the default strategy.
type machineAccountKeyStrategy struct{}

func (s *machineAccountKeyStrategy) NamespaceScoped() bool                                         { return true }
func (s *machineAccountKeyStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object)      {}
func (s *machineAccountKeyStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {}
func (s *machineAccountKeyStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return nil
}
func (s *machineAccountKeyStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}
func (s *machineAccountKeyStrategy) AllowCreateOnUpdate() bool       { return false }
func (s *machineAccountKeyStrategy) AllowUnconditionalUpdate() bool  { return false }
func (s *machineAccountKeyStrategy) Canonicalize(obj runtime.Object) {}
func (s *machineAccountKeyStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	// Disallow any updates (including PATCH and APPLY) to MachineAccountKey resources. They can only be created once
	// and subsequently deleted. Returning a Forbidden error here makes the apiserver reject any mutation attempts.
	return field.ErrorList{
		field.Forbidden(field.NewPath("metadata"), "updates to MachineAccountKey are forbidden; the resource is immutable and can only be deleted"),
	}
}
func (s *machineAccountKeyStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}
func (s *machineAccountKeyStrategy) GenerateName(base string) string {
	return names.SimpleNameGenerator.GenerateName(base)
}

func (s *machineAccountKeyStrategy) Recognizes(gvk schema.GroupVersionKind) bool {
	// Accept MachineAccountKey objects
	return gvk.Group == v1alpha1.SchemeGroupVersion.Group && gvk.Kind == "MachineAccountKey"
}

// ObjectKinds returns the kinds of objects this strategy handles
func (s *machineAccountKeyStrategy) ObjectKinds(obj runtime.Object) ([]schema.GroupVersionKind, bool, error) {
	return []schema.GroupVersionKind{
		{
			Group:   v1alpha1.SchemeGroupVersion.Group,
			Version: v1alpha1.SchemeGroupVersion.Version,
			Kind:    "MachineAccountKey",
		},
	}, false, nil
}
