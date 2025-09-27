package sessions

import (
	"context"
	"time"

	identityv1alpha1 "go.miloapis.com/milo/pkg/apis/identity/v1alpha1"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	authuser "k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

type Backend interface {
	ListSessions(ctx context.Context, u authuser.Info, opts *metav1.ListOptions) (*identityv1alpha1.SessionList, error)
	GetSession(ctx context.Context, u authuser.Info, name string) (*identityv1alpha1.Session, error)
	DeleteSession(ctx context.Context, u authuser.Info, name string) error
}

type REST struct {
	backend Backend
}

var _ rest.Scoper = &REST{}
var _ rest.Lister = &REST{}
var _ rest.Getter = &REST{}
var _ rest.GracefulDeleter = &REST{}
var _ rest.Storage = &REST{}
var _ rest.SingularNameProvider = &REST{}

func NewREST(b Backend) *REST { return &REST{backend: b} }

func (r *REST) GetSingularName() string { return "session" }
func (r *REST) NamespaceScoped() bool   { return false }
func (r *REST) New() runtime.Object     { return &identityv1alpha1.Session{} }
func (r *REST) NewList() runtime.Object { return &identityv1alpha1.SessionList{} }

func (r *REST) List(ctx context.Context, opts *metainternalversion.ListOptions) (runtime.Object, error) {
	u, _ := apirequest.UserFrom(ctx)
	// ignore selectors; self-scoped list delegated to provider
	lo := metav1.ListOptions{}
	return r.backend.ListSessions(ctx, u, &lo)
}

func (r *REST) Get(ctx context.Context, name string, _ *metav1.GetOptions) (runtime.Object, error) {
	u, _ := apirequest.UserFrom(ctx)
	return r.backend.GetSession(ctx, u, name)
}

func (r *REST) Delete(ctx context.Context, name string, _ rest.ValidateObjectFunc, _ *metav1.DeleteOptions) (runtime.Object, bool, error) {
	u, _ := apirequest.UserFrom(ctx)
	if err := r.backend.DeleteSession(ctx, u, name); err != nil {
		return nil, false, err
	}
	return &metav1.Status{Status: metav1.StatusSuccess}, true, nil
}

func (r *REST) Destroy() {}

// Satisfy rest.TableConvertor with a no-op conversion (returning nil uses default table printer)
func (r *REST) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	table := &metav1.Table{
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{Name: "Name", Type: "string"},
			{Name: "Provider", Type: "string"},
			{Name: "UserUID", Type: "string"},
			{Name: "Age", Type: "date"},
		},
	}

	appendRow := func(s *identityv1alpha1.Session) {
		age := metav1.Now().Rfc3339Copy()
		if !s.CreationTimestamp.IsZero() {
			// age shown as since creation, consistent with kubectl
			// metav1.Table wants a date in the cell; pass the timestamp
			age = s.CreationTimestamp
		}
		table.Rows = append(table.Rows, metav1.TableRow{
			Cells:  []interface{}{s.Name, s.Status.Provider, s.Status.UserUID, age.Time.Format(time.RFC3339)},
			Object: runtime.RawExtension{Object: s},
		})
	}

	switch obj := object.(type) {
	case *identityv1alpha1.SessionList:
		for i := range obj.Items {
			s := &obj.Items[i]
			appendRow(s)
		}
	case *identityv1alpha1.Session:
		appendRow(obj)
	default:
		// Fallback to default printer
		return nil, nil
	}

	return table, nil
}
