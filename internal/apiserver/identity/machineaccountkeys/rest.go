package machineaccountkeys

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	authuser "k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	identityv1alpha1 "go.miloapis.com/milo/pkg/apis/identity/v1alpha1"
)

// Backend is the interface that the REST handler delegates all operations to.
// Implementations proxy requests to the auth-provider (e.g. Zitadel) service.
type Backend interface {
	CreateMachineAccountKey(ctx context.Context, u authuser.Info, key *identityv1alpha1.MachineAccountKey, opts *metav1.CreateOptions) (*identityv1alpha1.MachineAccountKey, error)
	ListMachineAccountKeys(ctx context.Context, u authuser.Info, opts *metav1.ListOptions) (*identityv1alpha1.MachineAccountKeyList, error)
	GetMachineAccountKey(ctx context.Context, u authuser.Info, name string) (*identityv1alpha1.MachineAccountKey, error)
	DeleteMachineAccountKey(ctx context.Context, u authuser.Info, name string) error
}

type REST struct {
	backend Backend
}

var _ rest.Scoper = &REST{}
var _ rest.Creater = &REST{} //nolint:misspell
var _ rest.Lister = &REST{}
var _ rest.Getter = &REST{}
var _ rest.GracefulDeleter = &REST{}
var _ rest.Storage = &REST{}
var _ rest.SingularNameProvider = &REST{}

func NewREST(b Backend) *REST { return &REST{backend: b} }

func (r *REST) GetSingularName() string { return "machineaccountkey" }
func (r *REST) NamespaceScoped() bool   { return false }
func (r *REST) New() runtime.Object     { return &identityv1alpha1.MachineAccountKey{} }
func (r *REST) NewList() runtime.Object { return &identityv1alpha1.MachineAccountKeyList{} }

func (r *REST) Create(
	ctx context.Context,
	obj runtime.Object,
	_ rest.ValidateObjectFunc,
	opts *metav1.CreateOptions,
) (runtime.Object, error) {
	logger := klog.FromContext(ctx)
	u, _ := apirequest.UserFrom(ctx)
	key, ok := obj.(*identityv1alpha1.MachineAccountKey)
	if !ok {
		return nil, apierrors.NewBadRequest("not a MachineAccountKey")
	}
	logger.V(4).Info("Creating machine account key", "name", key.Name, "machineAccount", key.Spec.MachineAccountUserName)
	res, err := r.backend.CreateMachineAccountKey(ctx, u, key, opts)
	if err != nil {
		logger.Error(err, "Create machine account key failed", "name", key.Name)
		return nil, err
	}
	logger.V(4).Info("Created machine account key", "name", res.Name, "authProviderKeyID", res.Status.AuthProviderKeyID)
	return res, nil
}

func (r *REST) List(ctx context.Context, opts *metainternalversion.ListOptions) (runtime.Object, error) {
	logger := klog.FromContext(ctx)
	u, _ := apirequest.UserFrom(ctx)
	username, uid, groups := "", "", []string(nil)
	if u != nil {
		username = u.GetName()
		uid = u.GetUID()
		groups = u.GetGroups()
	}
	logger.V(4).Info("Listing machine account keys", "username", username, "uid", uid, "groups", groups)
	lo := metav1.ListOptions{}
	if opts != nil && opts.FieldSelector != nil && !opts.FieldSelector.Empty() {
		lo.FieldSelector = opts.FieldSelector.String()
	}
	res, err := r.backend.ListMachineAccountKeys(ctx, u, &lo)
	if err != nil {
		logger.Error(err, "List machine account keys failed")
		return nil, err
	}
	logger.V(4).Info("Listed machine account keys", "count", len(res.Items))
	return res, nil
}

func (r *REST) Get(ctx context.Context, name string, _ *metav1.GetOptions) (runtime.Object, error) {
	logger := klog.FromContext(ctx)
	u, _ := apirequest.UserFrom(ctx)
	username, uid := "", ""
	if u != nil {
		username = u.GetName()
		uid = u.GetUID()
	}
	logger.V(4).Info("Getting machine account key", "name", name, "username", username, "uid", uid)
	res, err := r.backend.GetMachineAccountKey(ctx, u, name)
	if err != nil {
		logger.Error(err, "Get machine account key failed", "name", name)
		return nil, err
	}
	logger.V(4).Info("Got machine account key", "name", name, "authProviderKeyID", res.Status.AuthProviderKeyID)
	return res, nil
}

func (r *REST) Delete(ctx context.Context, name string, _ rest.ValidateObjectFunc, _ *metav1.DeleteOptions) (runtime.Object, bool, error) {
	logger := klog.FromContext(ctx)
	u, _ := apirequest.UserFrom(ctx)
	username, uid := "", ""
	if u != nil {
		username = u.GetName()
		uid = u.GetUID()
	}
	logger.V(4).Info("Deleting machine account key", "name", name, "username", username, "uid", uid)
	if err := r.backend.DeleteMachineAccountKey(ctx, u, name); err != nil {
		logger.Error(err, "Delete machine account key failed", "name", name)
		return nil, false, err
	}
	logger.V(4).Info("Deleted machine account key", "name", name)
	return &metav1.Status{Status: metav1.StatusSuccess}, true, nil
}

func (r *REST) Destroy() {}

// ConvertToTable satisfies rest.TableConvertor with a kubectl-friendly table output.
func (r *REST) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	table := &metav1.Table{
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{Name: "Name", Type: "string"},
			{Name: "Machine Account", Type: "string"},
			{Name: "Key ID", Type: "string"},
			{Name: "Age", Type: "date"},
			{Name: "Expires", Type: "string"},
		},
	}

	appendRow := func(mak *identityv1alpha1.MachineAccountKey) {
		age := metav1.Now().Rfc3339Copy()
		if !mak.CreationTimestamp.IsZero() {
			age = mak.CreationTimestamp
		}
		expiresStr := "<none>"
		if mak.Spec.ExpirationDate != nil {
			expiresStr = mak.Spec.ExpirationDate.Time.Format(time.RFC3339)
		}
		table.Rows = append(table.Rows, metav1.TableRow{
			Cells:  []interface{}{mak.Name, mak.Spec.MachineAccountUserName, mak.Status.AuthProviderKeyID, age.Time.Format(time.RFC3339), expiresStr},
			Object: runtime.RawExtension{Object: mak},
		})
	}

	switch obj := object.(type) {
	case *identityv1alpha1.MachineAccountKeyList:
		for i := range obj.Items {
			appendRow(&obj.Items[i])
		}
	case *identityv1alpha1.MachineAccountKey:
		appendRow(obj)
	default:
		return nil, nil
	}

	return table, nil
}
