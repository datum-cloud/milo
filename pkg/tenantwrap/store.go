package tenantwrap

import (
	"context"
	"path"
	"reflect"
	"strings"

	"go.miloapis.com/milo/pkg/request"

	"k8s.io/klog/v2"

	"k8s.io/apiserver/pkg/registry/generic/registry"
)

// extractStore returns the embedded *genericregistry.Store if the object
// either IS a Store or HAS a field named “Store” that is one.
func ExtractStore(obj interface{}) *registry.Store {
	if s, ok := obj.(*registry.Store); ok {
		return s
	}
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	f := v.FieldByName("Store")
	if !f.IsValid() || f.IsZero() {
		return nil
	}
	if s, ok := f.Interface().(*registry.Store); ok {
		return s
	}
	return nil
}

// wrap rewrites the key builders so they add /projects/<id>/ in front.
func Wrap(s *registry.Store) {
	origRoot := s.KeyRootFunc
	origKey := s.KeyFunc

	s.KeyRootFunc = func(ctx context.Context) string {
		root := strings.TrimPrefix(origRoot(ctx), "/") // "registry/…"
		if proj, ok := request.ProjectID(ctx); ok && proj != "" {
			klog.V(4).Infof("[tenant] KeyRootFunc sees project=%q", proj)

			return path.Join("/projects", proj, root)
		}
		return "/" + root
	}

	s.KeyFunc = func(ctx context.Context, name string) (string, error) {
		key, err := origKey(ctx, name)
		if err != nil {
			return "", err
		}
		key = strings.TrimPrefix(key, "/")
		if proj, ok := request.ProjectID(ctx); ok && proj != "" {
			klog.V(4).Infof("[tenant] KeyFunc for namespaces sees project=%q", proj)
			return path.Join("/projects", proj, key), nil
		}
		return "/" + key, nil
	}
}
