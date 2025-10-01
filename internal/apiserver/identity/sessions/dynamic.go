package sessions

import (
	"context"
	"fmt"
	"strings"
	"time"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	identityv1alpha1 "go.miloapis.com/milo/pkg/apis/identity/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	authuser "k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Config struct {
	BaseConfig             *rest.Config
	ProviderGVR            schema.GroupVersionResource
	Timeout                time.Duration
	Retries                int
	ImpersonateExtrasAllow map[string]struct{}
}

type DynamicProvider struct {
	base        *rest.Config
	gvr         schema.GroupVersionResource
	to          time.Duration
	retries     int
	allowExtras map[string]struct{}
}

func NewDynamicProvider(cfg Config) (*DynamicProvider, error) {
	if cfg.BaseConfig == nil {
		return nil, fmt.Errorf("base rest.Config is required")
	}
	return &DynamicProvider{
		base:        rest.CopyConfig(cfg.BaseConfig),
		gvr:         cfg.ProviderGVR,
		to:          cfg.Timeout,
		retries:     cfg.Retries,
		allowExtras: cfg.ImpersonateExtrasAllow,
	}, nil
}

func (b *DynamicProvider) dynForUser(ctx context.Context) (dynamic.Interface, error) {
	u, ok := apirequest.UserFrom(ctx)
	if !ok || u == nil {
		return nil, fmt.Errorf("no user in context")
	}
	cfg := rest.CopyConfig(b.base)
	if b.to > 0 {
		cfg.Timeout = b.to
	}
	extras := map[string][]string{}
	for k, v := range u.GetExtra() {
		if _, ok := b.allowExtras[k]; ok {
			extras[k] = v
		}
	}

	// If this request arrived via the user-scoped control-plane virtual path,
	// reconstruct the same prefix for outgoing loopback calls by extending the
	// Host with the virtual workspace path. This mirrors how other controllers
	// in this repo scope clients (via Host path suffix).
	if pg := first(u.GetExtra()[iamv1alpha1.ParentAPIGroupExtraKey]); pg == iamv1alpha1.SchemeGroupVersion.Group {
		if pk := first(u.GetExtra()[iamv1alpha1.ParentKindExtraKey]); strings.EqualFold(pk, "User") {
			if pn := first(u.GetExtra()[iamv1alpha1.ParentNameExtraKey]); pn != "" {
				prefix := "/apis/" + iamv1alpha1.SchemeGroupVersion.Group + "/" + iamv1alpha1.SchemeGroupVersion.Version + "/users/" + pn + "/control-plane"
				cfg.Host = strings.TrimSuffix(cfg.Host, "/") + prefix
			}
		}
	}
	cfg.Impersonate = rest.ImpersonationConfig{
		UserName: u.GetName(),
		UID:      u.GetUID(),
		Groups:   u.GetGroups(),
		Extra:    extras,
	}

	return dynamic.NewForConfig(cfg)
}

func (b *DynamicProvider) ListSessions(ctx context.Context, _ authuser.Info, opts *metav1.ListOptions) (*identityv1alpha1.SessionList, error) {
	dyn, err := b.dynForUser(ctx)
	if err != nil {
		return nil, err
	}
	var ul *unstructured.UnstructuredList
	var lastErr error
	for i := 0; i <= b.retries; i++ {
		ul, lastErr = dyn.Resource(b.gvr).List(ctx, *opts)
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	out := new(identityv1alpha1.SessionList)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(ul.UnstructuredContent(), out); err != nil {
		return nil, err
	}
	return out, nil
}

func (b *DynamicProvider) GetSession(ctx context.Context, _ authuser.Info, name string) (*identityv1alpha1.Session, error) {
	dyn, err := b.dynForUser(ctx)
	if err != nil {
		return nil, err
	}
	var uobj *unstructured.Unstructured
	var lastErr error
	for i := 0; i <= b.retries; i++ {
		uobj, lastErr = dyn.Resource(b.gvr).Get(ctx, name, metav1.GetOptions{})
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	out := new(identityv1alpha1.Session)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(uobj.UnstructuredContent(), out); err != nil {
		return nil, err
	}
	return out, nil
}

func (b *DynamicProvider) DeleteSession(ctx context.Context, _ authuser.Info, name string) error {
	dyn, err := b.dynForUser(ctx)
	if err != nil {
		return err
	}
	var lastErr error
	for i := 0; i <= b.retries; i++ {
		lastErr = dyn.Resource(b.gvr).Delete(ctx, name, metav1.DeleteOptions{})
		if lastErr == nil {
			break
		}
	}
	return lastErr
}

// first returns the first element of a slice or "" if empty.
func first(v []string) string {
	if len(v) > 0 {
		return v[0]
	}
	return ""
}
