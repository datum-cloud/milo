package machineaccountkeys

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	identityv1alpha1 "go.miloapis.com/milo/pkg/apis/identity/v1alpha1"
)

// mockStore is a minimal fake underlying store for Create. It captures what
// object was passed to Create so tests can verify the private key was not persisted.
type mockStore struct {
	capturedObj runtime.Object
	returnObj   runtime.Object
	returnErr   error
}

func (m *mockStore) Create(
	_ context.Context,
	obj runtime.Object,
	_ rest.ValidateObjectFunc,
	_ *metav1.CreateOptions,
) (runtime.Object, error) {
	// Deep-copy to capture the state at the time of the store call.
	m.capturedObj = obj.DeepCopyObject()
	if m.returnErr != nil {
		return nil, m.returnErr
	}
	if m.returnObj != nil {
		return m.returnObj, nil
	}
	return obj, nil
}

// restWithMockStore builds a REST instance whose embedded Store.Create is replaced
// by a lightweight test double. We bypass NewREST to avoid needing an etcd backend
// in unit tests; only the Create interception logic is under test.
type testREST struct {
	store *mockStore
}

func (r *testREST) Create(
	ctx context.Context,
	obj runtime.Object,
	createValidation rest.ValidateObjectFunc,
	options *metav1.CreateOptions,
) (runtime.Object, error) {
	key, ok := obj.(*identityv1alpha1.MachineAccountKey)
	if !ok {
		return nil, apierrors.NewBadRequest("not a MachineAccountKey")
	}

	if key.Spec.ExpirationDate != nil && !key.Spec.ExpirationDate.Time.IsZero() {
		if !key.Spec.ExpirationDate.Time.After(time.Now()) {
			return nil, apierrors.NewBadRequest("spec.expirationDate must be in the future")
		}
	}

	if key.Spec.PublicKey != "" {
		if err := validateRSAPublicKey(key.Spec.PublicKey); err != nil {
			return nil, err
		}
	}

	var privateKeyPEM string
	if key.Spec.PublicKey == "" {
		pubPEM, privPEM, err := generateRSAKeyPairFunc()
		if err != nil {
			return nil, apierrors.NewInternalError(err)
		}
		key.Spec.PublicKey = pubPEM
		privateKeyPEM = privPEM
	}

	// Simulate strategy PrepareForCreate: clear private key before storage write.
	key.Status.PrivateKey = ""

	result, err := r.store.Create(ctx, key, createValidation, options)
	if err != nil {
		return nil, err
	}

	if privateKeyPEM != "" {
		createdKey, ok := result.(*identityv1alpha1.MachineAccountKey)
		if ok {
			createdKey.Status.PrivateKey = privateKeyPEM
		}
	}

	return result, nil
}

func newTestREST(store *mockStore) *testREST {
	return &testREST{store: store}
}

func makeKey(name, machineAccountName, publicKey string) *identityv1alpha1.MachineAccountKey {
	return &identityv1alpha1.MachineAccountKey{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: identityv1alpha1.MachineAccountKeySpec{
			MachineAccountName: machineAccountName,
			PublicKey:          publicKey,
		},
	}
}

func generateTestRSAPublicKeyPEM(t *testing.T) string {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
}

func TestCreate_AutoGeneration(t *testing.T) {
	store := &mockStore{}
	r := newTestREST(store)

	key := makeKey("test-key", "my-machine-account", "")

	ctx := apirequest.WithNamespace(context.Background(), "default")
	result, err := r.Create(ctx, key, nil, &metav1.CreateOptions{})

	require.NoError(t, err)
	require.NotNil(t, result)

	createdKey, ok := result.(*identityv1alpha1.MachineAccountKey)
	require.True(t, ok)

	// The response must contain a private key.
	assert.NotEmpty(t, createdKey.Status.PrivateKey, "private key should appear in the creation response")
	assert.Contains(t, createdKey.Status.PrivateKey, "-----BEGIN RSA PRIVATE KEY-----",
		"private key should be PEM-encoded PKCS1")

	// The spec must contain a public key.
	assert.NotEmpty(t, createdKey.Spec.PublicKey, "public key should be populated in spec")
	assert.Contains(t, createdKey.Spec.PublicKey, "-----BEGIN PUBLIC KEY-----")
}

func TestCreate_AutoGeneration_PrivateKeyAbsentFromStoredObject(t *testing.T) {
	store := &mockStore{}
	r := newTestREST(store)

	key := makeKey("test-key", "my-machine-account", "")

	ctx := apirequest.WithNamespace(context.Background(), "default")
	_, err := r.Create(ctx, key, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	// The object captured by the store (i.e., what would go to etcd) must have
	// no private key — this is the critical security invariant.
	stored, ok := store.capturedObj.(*identityv1alpha1.MachineAccountKey)
	require.True(t, ok)
	assert.Empty(t, stored.Status.PrivateKey,
		"private key must NOT be present in the object passed to storage (etcd)")
}

func TestCreate_ProvidedPublicKey_NoPrivateKeyInResponse(t *testing.T) {
	pubKeyPEM := generateTestRSAPublicKeyPEM(t)

	store := &mockStore{}
	r := newTestREST(store)

	key := makeKey("test-key", "my-machine-account", pubKeyPEM)

	ctx := apirequest.WithNamespace(context.Background(), "default")
	result, err := r.Create(ctx, key, nil, &metav1.CreateOptions{})

	require.NoError(t, err)
	require.NotNil(t, result)

	createdKey, ok := result.(*identityv1alpha1.MachineAccountKey)
	require.True(t, ok)

	// No private key should appear when the caller supplies their own public key.
	assert.Empty(t, createdKey.Status.PrivateKey,
		"status.privateKey should be absent when publicKey is provided")

	// The provided public key should be preserved unchanged.
	assert.Equal(t, pubKeyPEM, createdKey.Spec.PublicKey,
		"provided public key should be passed through unchanged")
}

func TestCreate_GenerationFailure_Returns500(t *testing.T) {
	// Override the key generation function to simulate a failure.
	original := generateRSAKeyPairFunc
	defer func() { generateRSAKeyPairFunc = original }()
	generateRSAKeyPairFunc = func() (string, string, error) {
		return "", "", errors.New("entropy source unavailable")
	}

	store := &mockStore{}
	r := newTestREST(store)

	key := makeKey("test-key", "my-machine-account", "")

	ctx := apirequest.WithNamespace(context.Background(), "default")
	_, err := r.Create(ctx, key, nil, &metav1.CreateOptions{})

	require.Error(t, err)
	statusErr := &apierrors.StatusError{}
	require.True(t, errors.As(err, &statusErr))
	assert.Equal(t, int32(500), statusErr.ErrStatus.Code,
		"key generation failure should return 500 Internal Server Error")
}

func TestCreate_StorageFailure_Propagated(t *testing.T) {
	expectedErr := errors.New("etcd connection refused")
	store := &mockStore{returnErr: expectedErr}
	r := newTestREST(store)

	key := makeKey("test-key", "my-machine-account", "")

	ctx := apirequest.WithNamespace(context.Background(), "default")
	_, err := r.Create(ctx, key, nil, &metav1.CreateOptions{})

	require.Error(t, err)
	assert.Equal(t, expectedErr, err, "storage errors should propagate unchanged")
}

func TestCreate_MalformedPublicKey_Returns400(t *testing.T) {
	store := &mockStore{}
	r := newTestREST(store)

	key := makeKey("test-key", "my-machine-account", "not-a-valid-pem")

	ctx := apirequest.WithNamespace(context.Background(), "default")
	_, err := r.Create(ctx, key, nil, &metav1.CreateOptions{})

	require.Error(t, err)
	statusErr := &apierrors.StatusError{}
	require.True(t, errors.As(err, &statusErr))
	assert.Equal(t, int32(400), statusErr.ErrStatus.Code)
}

func TestCreate_CorruptPublicKeyDER_Returns400(t *testing.T) {
	// Build a PEM block that decodes correctly but contains truncated DER bytes,
	// causing x509.ParsePKIXPublicKey to fail. This exercises the "invalid public key"
	// error path in validateRSAPublicKey.
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)

	corruptPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER[:len(pubDER)/2], // truncate to corrupt the DER
	}))

	store := &mockStore{}
	r := newTestREST(store)

	key := makeKey("test-key", "my-machine-account", corruptPEM)

	ctx := apirequest.WithNamespace(context.Background(), "default")
	_, createErr := r.Create(ctx, key, nil, &metav1.CreateOptions{})

	require.Error(t, createErr)
	statusErr := &apierrors.StatusError{}
	require.True(t, errors.As(createErr, &statusErr))
	assert.Equal(t, int32(400), statusErr.ErrStatus.Code)
}

func TestCreate_ExpirationDateInPast_Returns400(t *testing.T) {
	store := &mockStore{}
	r := newTestREST(store)

	pastTime := metav1.NewTime(time.Now().Add(-24 * time.Hour))
	key := makeKey("test-key", "my-machine-account", "")
	key.Spec.ExpirationDate = &pastTime

	ctx := apirequest.WithNamespace(context.Background(), "default")
	_, err := r.Create(ctx, key, nil, &metav1.CreateOptions{})

	require.Error(t, err)
	statusErr := &apierrors.StatusError{}
	require.True(t, errors.As(err, &statusErr))
	assert.Equal(t, int32(400), statusErr.ErrStatus.Code)
	assert.Contains(t, statusErr.ErrStatus.Message, "expirationDate")
}

func TestCreate_ExpirationDateInFuture_Succeeds(t *testing.T) {
	store := &mockStore{}
	r := newTestREST(store)

	futureTime := metav1.NewTime(time.Now().Add(24 * time.Hour))
	key := makeKey("test-key", "my-machine-account", "")
	key.Spec.ExpirationDate = &futureTime

	ctx := apirequest.WithNamespace(context.Background(), "default")
	result, err := r.Create(ctx, key, nil, &metav1.CreateOptions{})

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestValidateRSAPublicKey_ValidKey_ReturnsNil(t *testing.T) {
	pubKeyPEM := generateTestRSAPublicKeyPEM(t)
	err := validateRSAPublicKey(pubKeyPEM)
	assert.NoError(t, err)
}

func TestValidateRSAPublicKey_NotPEM_Returns400(t *testing.T) {
	err := validateRSAPublicKey("this is not PEM")
	require.Error(t, err)
	statusErr := &apierrors.StatusError{}
	require.True(t, errors.As(err, &statusErr))
	assert.Equal(t, int32(400), statusErr.ErrStatus.Code)
}

func TestValidateRSAPublicKey_CorruptDER_Returns400(t *testing.T) {
	corruptPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: []byte("not valid DER"),
	}))
	err := validateRSAPublicKey(corruptPEM)
	require.Error(t, err)
	statusErr := &apierrors.StatusError{}
	require.True(t, errors.As(err, &statusErr))
	assert.Equal(t, int32(400), statusErr.ErrStatus.Code)
}

func TestGenerateRSAKeyPair_ProducesValidKeys(t *testing.T) {
	pubPEM, privPEM, err := generateRSAKeyPair()
	require.NoError(t, err)

	// Validate public key.
	pubBlock, _ := pem.Decode([]byte(pubPEM))
	require.NotNil(t, pubBlock, "public key should be valid PEM")
	assert.Equal(t, "PUBLIC KEY", pubBlock.Type)
	pub, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	require.NoError(t, err)
	_, ok := pub.(*rsa.PublicKey)
	assert.True(t, ok, "public key should be RSA")

	// Validate private key.
	privBlock, _ := pem.Decode([]byte(privPEM))
	require.NotNil(t, privBlock, "private key should be valid PEM")
	assert.Equal(t, "RSA PRIVATE KEY", privBlock.Type)
	priv, err := x509.ParsePKCS1PrivateKey(privBlock.Bytes)
	require.NoError(t, err)
	assert.Equal(t, 2048, priv.Size()*8, "key should be 2048-bit")
}

func TestValidateUpdate_BlocksSpecChange(t *testing.T) {
	oldKey := makeKey("test-key", "original-account", "")
	newKey := makeKey("test-key", "changed-account", "")

	errs := Strategy.ValidateUpdate(context.Background(), newKey, oldKey)

	require.Len(t, errs, 1)
	assert.Equal(t, "spec", errs[0].Field)
	assert.Contains(t, errs[0].Detail, "immutable")
}

func TestValidateUpdate_BlocksPublicKeyRotation(t *testing.T) {
	oldPubKey := generateTestRSAPublicKeyPEM(t)
	newPubKey := generateTestRSAPublicKeyPEM(t)

	oldKey := makeKey("test-key", "my-account", oldPubKey)
	newKey := makeKey("test-key", "my-account", newPubKey)

	errs := Strategy.ValidateUpdate(context.Background(), newKey, oldKey)

	require.Len(t, errs, 1)
	assert.Equal(t, "spec", errs[0].Field)
	assert.Contains(t, errs[0].Detail, "immutable")
}

func TestValidateUpdate_BlocksPublicKeyToEmptyString(t *testing.T) {
	oldPubKey := generateTestRSAPublicKeyPEM(t)

	oldKey := makeKey("test-key", "my-account", oldPubKey)
	newKey := makeKey("test-key", "my-account", "")

	errs := Strategy.ValidateUpdate(context.Background(), newKey, oldKey)

	require.Len(t, errs, 1)
	assert.Equal(t, "spec", errs[0].Field)
	assert.Contains(t, errs[0].Detail, "immutable")
}

func TestValidateUpdate_NoChanges_Succeeds(t *testing.T) {
	pubKey := generateTestRSAPublicKeyPEM(t)
	expTime := metav1.NewTime(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))

	oldKey := makeKey("test-key", "my-account", pubKey)
	oldKey.Spec.ExpirationDate = &expTime

	newKey := makeKey("test-key", "my-account", pubKey)
	newKey.Spec.ExpirationDate = &expTime

	errs := Strategy.ValidateUpdate(context.Background(), newKey, oldKey)

	require.Len(t, errs, 0, "no-op update should succeed")
}
