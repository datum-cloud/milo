package sessions

import (
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	identityv1alpha1 "go.miloapis.com/milo/pkg/apis/identity/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/rest"
)

type recordingRT struct {
	lastPath string
}

func (r *recordingRT) RoundTrip(req *http.Request) (*http.Response, error) {
	r.lastPath = req.URL.Path
	body := ioutil.NopCloser(strings.NewReader(`{"kind":"SessionList","apiVersion":"identity.miloapis.com/v1alpha1","items":[]}`))
	return &http.Response{StatusCode: 200, Body: body, Header: make(http.Header)}, nil
}

func contextWithUser(u *user.DefaultInfo) context.Context {
	ctx := context.Background()
	return apirequest.WithUser(ctx, u)
}

func TestDynamicProvider_UserScopedAddsPrefix(t *testing.T) {
	// Arrange base config with recording transport
	rt := &recordingRT{}
	base := &rest.Config{Host: "http://example.org", Transport: rt}
	prov, err := NewDynamicProvider(Config{
		BaseConfig:  base,
		ProviderGVR: schema.GroupVersionResource{Group: identityv1alpha1.GroupVersion.Group, Version: identityv1alpha1.GroupVersion.Version, Resource: "sessions"},
		Retries:     0,
	})
	if err != nil {
		t.Fatalf("NewDynamicProvider: %v", err)
	}

	// Build a user-scoped context (IAM user virtual workspace)
	u := &user.DefaultInfo{
		Name:   "alice",
		UID:    "12345",
		Groups: []string{"system:authenticated"},
		Extra: map[string][]string{
			iamv1alpha1.ParentAPIGroupExtraKey: {iamv1alpha1.SchemeGroupVersion.Group},
			iamv1alpha1.ParentKindExtraKey:     {"User"},
			iamv1alpha1.ParentNameExtraKey:     {"test-user-uid"},
		},
	}
	ctx := contextWithUser(u)

	// Act: list sessions
	_, err = prov.ListSessions(ctx, u, &metav1.ListOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	// Assert path has the user control-plane prefix
	expectedPrefix := "/apis/" + iamv1alpha1.SchemeGroupVersion.Group + "/" + iamv1alpha1.SchemeGroupVersion.Version + "/users/test-user-uid/control-plane"
	if !strings.HasPrefix(rt.lastPath, expectedPrefix+"/apis/"+identityv1alpha1.GroupVersion.Group+"/"+identityv1alpha1.GroupVersion.Version+"/sessions") {
		t.Fatalf("expected path to start with %q and sessions resource, got %q", expectedPrefix, rt.lastPath)
	}
}

func TestDynamicProvider_NonUserScopedNoPrefix(t *testing.T) {
	rt := &recordingRT{}
	base := &rest.Config{Host: "http://example.org", Transport: rt}
	prov, err := NewDynamicProvider(Config{
		BaseConfig:  base,
		ProviderGVR: schema.GroupVersionResource{Group: identityv1alpha1.GroupVersion.Group, Version: identityv1alpha1.GroupVersion.Version, Resource: "sessions"},
		Retries:     0,
	})
	if err != nil {
		t.Fatalf("NewDynamicProvider: %v", err)
	}

	// No IAM extras => should not add user control-plane prefix
	u := &user.DefaultInfo{Name: "bob", UID: "42", Groups: []string{"system:authenticated"}}
	ctx := contextWithUser(u)

	_, err = prov.ListSessions(ctx, u, &metav1.ListOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	expectedNoPrefix := "/apis/" + identityv1alpha1.GroupVersion.Group + "/" + identityv1alpha1.GroupVersion.Version + "/sessions"
	if !strings.HasPrefix(rt.lastPath, expectedNoPrefix) {
		t.Fatalf("expected path to start with %q, got %q", expectedNoPrefix, rt.lastPath)
	}
}
