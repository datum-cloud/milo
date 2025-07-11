package app

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/storage"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
)

// MachineAccountKeyStorageProvider is a RESTStorageProvider for MachineAccountKey resources.
type MachineAccountKeyStorageProvider struct {
	Scheme *runtime.Scheme
}

var _ interface {
	GroupName() string
	NewRESTStorage(storage.APIResourceConfigSource, generic.RESTOptionsGetter) (genericapiserver.APIGroupInfo, error)
} = &MachineAccountKeyStorageProvider{}

func (h *MachineAccountKeyStorageProvider) GroupName() string {
	return v1alpha1.SchemeGroupVersion.Group
}

func (h *MachineAccountKeyStorageProvider) NewRESTStorage(apiResourceConfigSource storage.APIResourceConfigSource, restOptionsGetter generic.RESTOptionsGetter) (genericapiserver.APIGroupInfo, error) {
	scheme := h.Scheme
	if scheme == nil {
		scheme = legacyscheme.Scheme
	}

	paramCodec := runtime.NewParameterCodec(scheme)
	codecFactory := serializer.NewCodecFactory(scheme)

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(
		v1alpha1.SchemeGroupVersion.Group,
		scheme,
		paramCodec,
		codecFactory,
	)

	storage := map[string]rest.Storage{
		"machineaccountkeys": NewMachineAccountKeyREST(scheme, restOptionsGetter),
	}
	apiGroupInfo.VersionedResourcesStorageMap[v1alpha1.SchemeGroupVersion.Version] = storage

	if !apiResourceConfigSource.ResourceEnabled(
		v1alpha1.SchemeGroupVersion.WithResource("machineaccountkeys")) {
		klog.Info("machineaccountkeys resource is DISABLED by APIResourceConfigSource")
	}

	return apiGroupInfo, nil
}
