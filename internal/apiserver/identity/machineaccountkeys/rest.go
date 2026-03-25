package machineaccountkeys

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	generic "k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	k8smetrics "k8s.io/component-base/metrics"
	k8slegacy "k8s.io/component-base/metrics/legacyregistry"

	identityv1alpha1 "go.miloapis.com/milo/pkg/apis/identity/v1alpha1"
)

// generateRSAKeyPairFunc is a package-level variable to allow test overriding.
var generateRSAKeyPairFunc = generateRSAKeyPair

// metrics for key generation observability (NFR3)
var (
	keyGenerationDuration = k8smetrics.NewHistogramVec(
		&k8smetrics.HistogramOpts{
			Name:           "milo_machineaccountkey_generation_duration_seconds",
			Help:           "Duration of RSA key generation for MachineAccountKey creation",
			Buckets:        []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
			StabilityLevel: k8smetrics.ALPHA,
		},
		[]string{"result"}, // "success" | "failure"
	)
)

func init() {
	k8slegacy.MustRegister(keyGenerationDuration)
}

// REST wraps a genericregistry.Store and intercepts Create to inject RSA key generation.
// It embeds the store to inherit all standard REST operations (Get, List, Watch, Delete,
// ConvertToTable, etc.) and only overrides Create.
type REST struct {
	*genericregistry.Store
}

var _ rest.Scoper = &REST{}
var _ rest.Creater = &REST{}
var _ rest.Updater = &REST{}
var _ rest.Getter = &REST{}
var _ rest.Lister = &REST{}
var _ rest.GracefulDeleter = &REST{}
var _ rest.Watcher = &REST{}
var _ rest.Storage = &REST{}
var _ rest.SingularNameProvider = &REST{}

// NewREST constructs a REST storage handler backed by etcd via the given RESTOptionsGetter.
// It returns an error if the store cannot be completed (e.g. RESTOptionsGetter is misconfigured).
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, error) {
	gr := identityv1alpha1.SchemeGroupVersion.WithResource("machineaccountkeys").GroupResource()

	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &identityv1alpha1.MachineAccountKey{} },
		NewListFunc:               func() runtime.Object { return &identityv1alpha1.MachineAccountKeyList{} },
		DefaultQualifiedResource:  gr,
		SingularQualifiedResource: identityv1alpha1.SchemeGroupVersion.WithResource("machineaccountkey").GroupResource(),
		CreateStrategy:            Strategy,
		UpdateStrategy:            Strategy,
		DeleteStrategy:            Strategy,
		TableConvertor:            rest.NewDefaultTableConvertor(gr),
	}

	options := &generic.StoreOptions{
		RESTOptions: optsGetter,
	}

	if err := store.CompleteWithOptions(options); err != nil {
		return nil, fmt.Errorf("failed to complete machineaccountkeys store: %w", err)
	}

	return &REST{Store: store}, nil
}

func (r *REST) GetSingularName() string { return "machineaccountkey" }

// Create intercepts the standard create path to optionally generate an RSA key pair
// when spec.publicKey is omitted. The private key is returned in status.privateKey
// of the HTTP response object only — it is never passed to etcd storage.
func (r *REST) Create(
	ctx context.Context,
	obj runtime.Object,
	createValidation rest.ValidateObjectFunc,
	options *metav1.CreateOptions,
) (runtime.Object, error) {
	key, ok := obj.(*identityv1alpha1.MachineAccountKey)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf(
			"not a MachineAccountKey: %T", obj,
		))
	}

	// FR6: validate expiration date is in the future if provided
	if key.Spec.ExpirationDate != nil && !key.Spec.ExpirationDate.Time.IsZero() {
		if !key.Spec.ExpirationDate.Time.After(time.Now()) {
			return nil, apierrors.NewBadRequest("spec.expirationDate must be in the future")
		}
	}

	// FR7: validate public key format if provided
	if key.Spec.PublicKey != "" {
		if err := validateRSAPublicKey(key.Spec.PublicKey); err != nil {
			return nil, err
		}
	}

	// FR1: auto-generate RSA 2048-bit key pair when no public key is provided
	var privateKeyPEM string
	if key.Spec.PublicKey == "" {
		start := time.Now()
		pubPEM, privPEM, err := generateRSAKeyPairFunc()
		elapsed := time.Since(start)

		if err != nil {
			keyGenerationDuration.WithLabelValues("failure").Observe(elapsed.Seconds())
			return nil, apierrors.NewInternalError(fmt.Errorf("failed to generate RSA key pair: %w", err))
		}
		keyGenerationDuration.WithLabelValues("success").Observe(elapsed.Seconds())

		key.Spec.PublicKey = pubPEM
		privateKeyPEM = privPEM
	}

	// Persist to etcd. The strategy's PrepareForCreate ensures Status.PrivateKey
	// is cleared before the object reaches storage (defense in depth).
	result, err := r.Store.Create(ctx, key, createValidation, options)
	if err != nil {
		return nil, err
	}

	// FR1/FR3: Set the private key on the in-memory response object AFTER the
	// etcd write returns. The object stored in etcd never carries the private key.
	if privateKeyPEM != "" {
		createdKey, ok := result.(*identityv1alpha1.MachineAccountKey)
		if ok {
			createdKey.Status.PrivateKey = privateKeyPEM
		}
	}

	return result, nil
}

// Update intercepts the standard update path to support key rotation.
// It allows updating the publicKey field and optionally auto-generates a new key pair
// if publicKey is set to empty string. The strategy's ValidateUpdate enforces immutability
// of machineAccountName and expirationDate.
func (r *REST) Update(
	ctx context.Context,
	name string,
	objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc,
	updateValidation rest.ValidateObjectUpdateFunc,
	forceAllowCreate bool,
	options *metav1.UpdateOptions,
) (runtime.Object, bool, error) {
	var privateKeyPEM string

	// Wrap objInfo to intercept the update and trigger rotation if publicKey is cleared.
	info := &rotationUpdatedObjectInfo{
		UpdatedObjectInfo: objInfo,
		update: func(ctx context.Context, obj, old runtime.Object) (runtime.Object, error) {
			newKey, ok := obj.(*identityv1alpha1.MachineAccountKey)
			if !ok {
				return obj, nil
			}

			// If the public key is explicitly cleared in the update, generate a new one.
			if newKey.Spec.PublicKey == "" {
				start := time.Now()
				pubPEM, privPEM, err := generateRSAKeyPairFunc()
				elapsed := time.Since(start)

				if err != nil {
					keyGenerationDuration.WithLabelValues("failure").Observe(elapsed.Seconds())
					return nil, apierrors.NewInternalError(err)
				}
				keyGenerationDuration.WithLabelValues("success").Observe(elapsed.Seconds())

				newKey.Spec.PublicKey = pubPEM
				privateKeyPEM = privPEM
			}
			return newKey, nil
		},
	}

	result, created, err := r.Store.Update(ctx, name, info, createValidation, updateValidation, forceAllowCreate, options)
	if err != nil {
		return nil, false, err
	}

	// Set the private key on the response object (in-memory only).
	if privateKeyPEM != "" {
		if updatedKey, ok := result.(*identityv1alpha1.MachineAccountKey); ok {
			updatedKey.Status.PrivateKey = privateKeyPEM
		}
	}

	return result, created, nil
}

// generateRSAKeyPair generates a 2048-bit RSA key pair and returns
// PEM-encoded public (PKIX) and private (PKCS1) key material.
func generateRSAKeyPair() (publicKeyPEM string, privateKeyPEM string, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("rsa.GenerateKey: %w", err)
	}

	pubDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("x509.MarshalPKIXPublicKey: %w", err)
	}

	pubPEMBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	})

	privDER := x509.MarshalPKCS1PrivateKey(privateKey)
	privPEMBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privDER,
	})

	return string(pubPEMBlock), string(privPEMBlock), nil
}

// validateRSAPublicKey returns an API error if the PEM string is not a valid
// PEM-encoded RSA public key. It returns nil when the key is acceptable.
func validateRSAPublicKey(pubKeyPEM string) error {
	block, _ := pem.Decode([]byte(pubKeyPEM))
	if block == nil {
		return apierrors.NewBadRequest("spec.publicKey must be PEM-encoded")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("invalid public key: %v", err))
	}

	if _, ok := pub.(*rsa.PublicKey); !ok {
		return apierrors.NewBadRequest("only RSA public keys are supported in v1alpha1")
	}

	return nil
}

// rotationUpdatedObjectInfo wraps rest.UpdatedObjectInfo to allow intercepting
// the resulting object and performing transformations (like key generation)
// before it is passed to the underlying store.
type rotationUpdatedObjectInfo struct {
	rest.UpdatedObjectInfo
	update func(ctx context.Context, obj, old runtime.Object) (runtime.Object, error)
}

func (i *rotationUpdatedObjectInfo) UpdatedObject(ctx context.Context, old runtime.Object) (runtime.Object, error) {
	newObj, err := i.UpdatedObjectInfo.UpdatedObject(ctx, old)
	if err != nil {
		return nil, err
	}
	return i.update(ctx, newObj, old)
}
