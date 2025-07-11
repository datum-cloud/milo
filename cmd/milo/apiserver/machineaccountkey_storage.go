package app

import (
	"context"
	"fmt"

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

	// 1. run the standard validation so you don’t lose that check
	if err := r.CreateStrategy.Validate(ctx, obj); len(err) != 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "iam.miloapis.com", Kind: "MachineAccountKey"},
			obj.(*v1alpha1.MachineAccountKey).Name, err)
	}

	// 2. persist to etcd (or skip if you truly don’t want persistence)
	createdObj, err := r.Store.Create(ctx, obj, validate, options)
	if err != nil {
		return nil, err
	}

	// 3. build *any* runtime.Object you want to send back.
	//    Here we construct an arbitrary JSON payload.
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
			"privateKey": "-----BEGIN…", // whatever you want to return
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
	return nil
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
