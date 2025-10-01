package lifecycle

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// helper to build a granted claim with no ownerRefs
func newGrantedClaim(ns, name string, refGroup, refKind, refName, refNS string, created time.Time) *quotav1alpha1.ResourceClaim {
	return &quotav1alpha1.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         ns,
			CreationTimestamp: metav1.Time{Time: created},
		},
		Spec: quotav1alpha1.ResourceClaimSpec{
			ConsumerRef: quotav1alpha1.ConsumerRef{
				APIGroup: "resourcemanager.miloapis.com",
				Kind:     "Organization",
				Name:     "test-org",
			},
			Requests: []quotav1alpha1.ResourceRequest{{
				ResourceType: "test-resource-type",
				Amount:       1,
			}},
			ResourceRef: quotav1alpha1.UnversionedObjectReference{
				APIGroup:  refGroup,
				Kind:      refKind,
				Name:      refName,
				Namespace: refNS,
			},
		},
		Status: quotav1alpha1.ResourceClaimStatus{
			Conditions: []metav1.Condition{{
				Type:   quotav1alpha1.ResourceClaimGranted,
				Status: metav1.ConditionTrue,
			}},
		},
	}
}

func restMapperForCoreSecrets() meta.RESTMapper {
	rm := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	rm.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}, meta.RESTScopeNamespace)
	return rm
}

func TestOwnership_FastPath_SetsOwnerRef(t *testing.T) {
	t.Skip("Fake client does not properly support server-side apply - see https://github.com/kubernetes-sigs/controller-runtime/issues/1464")

	scheme := runtime.NewScheme()
	_ = quotav1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// Owner exists
	owner := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "owner", Namespace: "ns1", UID: "u-1"}}

	// Claim granted, no ownerRefs
	claim := newGrantedClaim("ns1", "claim1", "", "Secret", "owner", "", time.Now())

	// Fake clients
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(claim).Build()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme, owner)

	r := &ResourceClaimOwnershipController{Client: k8sClient, DynamicClient: dynClient, Scheme: scheme, restMapper: restMapperForCoreSecrets()}

	// Reconcile
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(claim)})
	if err != nil && strings.Contains(err.Error(), "apply patches are not supported") {
		t.Skip("fake client does not support server-side apply")
	}
	if err != nil && strings.Contains(err.Error(), "server-side apply") {
		t.Skip("fake client does not support server-side apply")
	}
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Verify ownerRef was set
	var updated quotav1alpha1.ResourceClaim
	if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(claim), &updated); err != nil {
		t.Fatalf("get updated claim failed: %v", err)
	}
	if len(updated.OwnerReferences) != 1 {
		t.Fatalf("expected 1 ownerRef, got %d", len(updated.OwnerReferences))
	}
	or := updated.OwnerReferences[0]
	if or.Kind != "Secret" || or.Name != "owner" || or.Controller == nil || !*or.Controller {
		t.Fatalf("unexpected ownerRef: %+v", or)
	}
}

func TestOwnership_Requeues_WhenOwnerMissing_ThenSetsOwner(t *testing.T) {
	t.Skip("Fake client does not properly support server-side apply - see https://github.com/kubernetes-sigs/controller-runtime/issues/1464")

	scheme := runtime.NewScheme()
	_ = quotav1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	claim := newGrantedClaim("ns1", "claim2", "", "Secret", "owner2", "", time.Now())

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(claim).Build()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme /* no owner yet */)

	r := &ResourceClaimOwnershipController{Client: k8sClient, DynamicClient: dynClient, Scheme: scheme, restMapper: restMapperForCoreSecrets()}

	// First reconcile: owner missing -> expect no error and requeue implied by no SSA
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(claim)}); err != nil && !strings.Contains(err.Error(), "apply patches are not supported") && !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error on first reconcile: %v", err)
	}

	// Create the owner now
	owner := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "owner2", Namespace: "ns1", UID: "u-2"}}
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	if _, err := dynClient.Resource(gvr).Namespace("ns1").Create(context.Background(), toUnstructured(t, owner), metav1.CreateOptions{}); err != nil {
		t.Fatalf("failed creating owner in dynamic fake: %v", err)
	}

	// Second reconcile: should set ownerRef via SSA
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(claim)})
	if err != nil && strings.Contains(err.Error(), "apply patches are not supported") {
		t.Skip("fake client does not support server-side apply")
	}
	if err != nil && strings.Contains(err.Error(), "server-side apply") {
		t.Skip("fake client does not support server-side apply")
	}
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	var updated quotav1alpha1.ResourceClaim
	_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(claim), &updated)
	if len(updated.OwnerReferences) != 1 {
		t.Fatalf("expected 1 ownerRef after rescue, got %d", len(updated.OwnerReferences))
	}
}

func TestOwnership_DeletesAfterMaxAge_WhenOwnerNeverAppears(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = quotav1alpha1.AddToScheme(scheme)

	// Force short grace/max
	t.Setenv("RESOURCECLAIM_GRACE_PERIOD", "0s")
	t.Setenv("RESOURCECLAIM_MAX_ORPHAN_AGE", "1ms")

	old := time.Now().Add(-1 * time.Hour)
	claim := newGrantedClaim("ns1", "claim3", "", "Secret", "nope", "", old)

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(claim).Build()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme)

	r := &ResourceClaimOwnershipController{Client: k8sClient, DynamicClient: dynClient, Scheme: scheme, restMapper: restMapperForCoreSecrets()}

	// Reconcile should delete the claim since owner never appears and age > max
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(claim)}); err != nil && !strings.Contains(err.Error(), "not found") {
		// ignore not found due to eventual delete
		t.Fatalf("unexpected error: %v", err)
	}

	var check quotav1alpha1.ResourceClaim
	err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(claim), &check)
	if err == nil {
		t.Fatalf("expected claim to be deleted")
	}
}

func TestOwnership_Skips_WhenAlreadyOwnedOrNotGranted(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = quotav1alpha1.AddToScheme(scheme)

	// Not granted
	notGranted := &quotav1alpha1.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "ns1"},
		Spec: quotav1alpha1.ResourceClaimSpec{
			ResourceRef: quotav1alpha1.UnversionedObjectReference{APIGroup: "", Kind: "Secret", Name: "owner"},
		},
		Status: quotav1alpha1.ResourceClaimStatus{Conditions: []metav1.Condition{}},
	}

	// Already owned
	controller := true
	owned := newGrantedClaim("ns1", "c2", "", "Secret", "owner", "", time.Now())
	owned.OwnerReferences = []metav1.OwnerReference{{APIVersion: "v1", Kind: "Secret", Name: "owner", Controller: &controller}}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(notGranted, owned).Build()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme)

	r := &ResourceClaimOwnershipController{Client: k8sClient, DynamicClient: dynClient, Scheme: scheme, restMapper: restMapperForCoreSecrets()}

	// Not granted should no-op
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(notGranted)}); err != nil {
		t.Fatalf("not granted reconcile error: %v", err)
	}
	// Already owned should no-op
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(owned)}); err != nil {
		t.Fatalf("already owned reconcile error: %v", err)
	}
}

func toUnstructured(t *testing.T, obj runtime.Object) *unstructured.Unstructured {
	t.Helper()
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatalf("toUnstructured: %v", err)
	}
	return &unstructured.Unstructured{Object: m}
}
