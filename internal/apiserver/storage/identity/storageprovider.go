package identity

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	generic "k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	controlplaneapiserver "k8s.io/kubernetes/pkg/controlplane/apiserver"

	identregistry "go.miloapis.com/milo/internal/apiserver/identity/sessions"
	identityv1alpha1 "go.miloapis.com/milo/pkg/apis/identity/v1alpha1"
)

type StorageProvider struct {
	Sessions identregistry.Backend
}

func (p StorageProvider) GroupName() string { return identityv1alpha1.SchemeGroupVersion.Group }

func (p StorageProvider) NewRESTStorage(
	_ serverstorage.APIResourceConfigSource,
	_ generic.RESTOptionsGetter,
) (genericapiserver.APIGroupInfo, error) {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(
		identityv1alpha1.SchemeGroupVersion.Group,
		legacyscheme.Scheme,
		metav1.ParameterCodec,
		legacyscheme.Codecs,
	)

	storage := map[string]rest.Storage{
		"sessions": identregistry.NewREST(p.Sessions),
	}

	apiGroupInfo.VersionedResourcesStorageMap = map[string]map[string]rest.Storage{
		identityv1alpha1.SchemeGroupVersion.Version: storage,
	}

	return apiGroupInfo, nil
}

var _ controlplaneapiserver.RESTStorageProvider = StorageProvider{}
