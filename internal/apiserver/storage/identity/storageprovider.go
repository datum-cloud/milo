package identity

import (
	"k8s.io/apimachinery/pkg/runtime"
	generic "k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	controlplaneapiserver "k8s.io/kubernetes/pkg/controlplane/apiserver"

	sessionsregistry "go.miloapis.com/milo/internal/apiserver/identity/sessions"
	useridentitiesregistry "go.miloapis.com/milo/internal/apiserver/identity/useridentities"
	identityv1alpha1 "go.miloapis.com/milo/pkg/apis/identity/v1alpha1"
)

type StorageProvider struct {
	Sessions       sessionsregistry.Backend
	UserIdentities useridentitiesregistry.Backend
}

func (p StorageProvider) GroupName() string { return identityv1alpha1.SchemeGroupVersion.Group }

func (p StorageProvider) NewRESTStorage(
	_ serverstorage.APIResourceConfigSource,
	_ generic.RESTOptionsGetter,
) (genericapiserver.APIGroupInfo, error) {
	// Create ParameterCodec using legacyscheme.Scheme which has identity scheme installed
	// with field label conversion functions. This is critical for field selector validation.
	// The identity scheme is installed in cmd/milo/apiserver/config.go via identityapi.Install(legacyscheme.Scheme)
	parameterCodec := runtime.NewParameterCodec(legacyscheme.Scheme)

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(
		identityv1alpha1.SchemeGroupVersion.Group,
		legacyscheme.Scheme,
		parameterCodec,
		legacyscheme.Codecs,
	)

	storage := map[string]rest.Storage{
		"sessions":       sessionsregistry.NewREST(p.Sessions),
		"useridentities": useridentitiesregistry.NewREST(p.UserIdentities),
	}

	apiGroupInfo.VersionedResourcesStorageMap = map[string]map[string]rest.Storage{
		identityv1alpha1.SchemeGroupVersion.Version: storage,
	}

	return apiGroupInfo, nil
}

var _ controlplaneapiserver.RESTStorageProvider = StorageProvider{}
