package identity

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	generic "k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
	"k8s.io/klog/v2"
	controlplaneapiserver "k8s.io/kubernetes/pkg/controlplane/apiserver"

	sessionsregistry "go.miloapis.com/milo/internal/apiserver/identity/sessions"
	useridentitiesregistry "go.miloapis.com/milo/internal/apiserver/identity/useridentities"
	identityinstall "go.miloapis.com/milo/pkg/apis/identity/install"
	identityv1alpha1 "go.miloapis.com/milo/pkg/apis/identity/v1alpha1"
)

var (
	// Scheme defines the runtime type system for Identity API object serialization.
	Scheme = runtime.NewScheme()
	// Codecs provides serializers for Identity API objects.
	Codecs = serializer.NewCodecFactory(Scheme)
)

func init() {
	klog.Error("========== IDENTITY STORAGE INIT START ==========")

	identityinstall.Install(Scheme)
	klog.Error("Identity install complete")

	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})

	// Register unversioned meta types required by the API machinery.
	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)

	// Test if field label conversion functions are registered correctly
	userIdentityGVK := identityv1alpha1.SchemeGroupVersion.WithKind("UserIdentity")
	sessionGVK := identityv1alpha1.SchemeGroupVersion.WithKind("Session")

	klog.Errorf("Testing UserIdentity GVK: %s", userIdentityGVK.String())

	// Test UserIdentity field label conversion
	if _, _, err := Scheme.ConvertFieldLabel(userIdentityGVK, "status.userUID", "test"); err == nil {
		klog.Error("✓ UserIdentity field label conversion WORKS")
	} else {
		klog.Errorf("✗ UserIdentity field label conversion FAILED: %v", err)
	}

	// Test Session field label conversion
	if _, _, err := Scheme.ConvertFieldLabel(sessionGVK, "status.userUID", "test"); err == nil {
		klog.Error("✓ Session field label conversion WORKS")
	} else {
		klog.Errorf("✗ Session field label conversion FAILED: %v", err)
	}

	klog.Error("========== IDENTITY STORAGE INIT END ==========")
}

type StorageProvider struct {
	Sessions       sessionsregistry.Backend
	UserIdentities useridentitiesregistry.Backend
}

func (p StorageProvider) GroupName() string { return identityv1alpha1.SchemeGroupVersion.Group }

func (p StorageProvider) NewRESTStorage(
	_ serverstorage.APIResourceConfigSource,
	_ generic.RESTOptionsGetter,
) (genericapiserver.APIGroupInfo, error) {
	// Use dedicated Identity Scheme with field label conversion functions registered
	// This follows the same pattern as Activity API
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(
		identityv1alpha1.SchemeGroupVersion.Group,
		Scheme,
		metav1.ParameterCodec,
		Codecs,
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
