// Copyright 2024 The Milo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"

	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
)

func TestFilterGroupItems(t *testing.T) {
	f := NewContactGroupVisibilityFilter(nil)

	tests := []struct {
		name             string
		groups           []notificationv1alpha1.ContactGroup
		accessibleGroups map[string]bool
		hasContact       bool
		expectedNames    []string
	}{
		{
			name: "public groups are always visible regardless of contact status",
			groups: []notificationv1alpha1.ContactGroup{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "group1", Namespace: "default"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPublic},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "group2", Namespace: "default"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPublic},
				},
			},
			accessibleGroups: nil,
			hasContact:       false,
			expectedNames:    []string{"group1", "group2"},
		},
		{
			name: "private groups visible when user has membership",
			groups: []notificationv1alpha1.ContactGroup{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "public-group", Namespace: "default"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPublic},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "private-group", Namespace: "default"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPrivate},
				},
			},
			accessibleGroups: map[string]bool{"default/private-group": true},
			hasContact:       true,
			expectedNames:    []string{"public-group", "private-group"},
		},
		{
			name: "private groups filtered when user has no membership",
			groups: []notificationv1alpha1.ContactGroup{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "public-group", Namespace: "default"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPublic},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "private-group", Namespace: "default"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPrivate},
				},
			},
			accessibleGroups: map[string]bool{}, // user has contact but no memberships
			hasContact:       true,
			expectedNames:    []string{"public-group"},
		},
		{
			name: "all private groups filtered when user has no contact",
			groups: []notificationv1alpha1.ContactGroup{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "private1", Namespace: "default"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPrivate},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "private2", Namespace: "default"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPrivate},
				},
			},
			accessibleGroups: nil,
			hasContact:       false,
			expectedNames:    []string{},
		},
		{
			name: "mixed visibility with partial membership",
			groups: []notificationv1alpha1.ContactGroup{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "public-1", Namespace: "ns1"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPublic},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "private-accessible", Namespace: "ns1"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPrivate},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "private-not-accessible", Namespace: "ns2"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPrivate},
				},
			},
			accessibleGroups: map[string]bool{"ns1/private-accessible": true},
			hasContact:       true,
			expectedNames:    []string{"public-1", "private-accessible"},
		},
		{
			name:             "empty list remains empty",
			groups:           []notificationv1alpha1.ContactGroup{},
			accessibleGroups: nil,
			hasContact:       false,
			expectedNames:    []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filtered := f.filterGroupItems(tc.groups, tc.accessibleGroups, tc.hasContact)
			if len(filtered) != len(tc.expectedNames) {
				t.Errorf("expected %d groups, got %d", len(tc.expectedNames), len(filtered))
			}

			// Verify expected groups are present
			for i, expectedName := range tc.expectedNames {
				if i >= len(filtered) {
					t.Errorf("missing expected group %s", expectedName)
					continue
				}
				if filtered[i].Name != expectedName {
					t.Errorf("expected group %s at position %d, got %s", expectedName, i, filtered[i].Name)
				}
			}
		})
	}
}

func TestCaptureResponseWriter(t *testing.T) {
	t.Run("captures body and status", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		capture := newCaptureResponseWriter(recorder)

		capture.WriteHeader(http.StatusCreated)
		_, _ = capture.Write([]byte("test body"))

		if capture.statusCode != http.StatusCreated {
			t.Errorf("expected status %d, got %d", http.StatusCreated, capture.statusCode)
		}
		if string(capture.body) != "test body" {
			t.Errorf("expected body %q, got %q", "test body", string(capture.body))
		}
	})

	t.Run("flush writes to underlying writer", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		capture := newCaptureResponseWriter(recorder)

		capture.WriteHeader(http.StatusOK)
		_, _ = capture.Write([]byte("flushed content"))
		capture.flush()

		if recorder.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, recorder.Code)
		}
		if recorder.Body.String() != "flushed content" {
			t.Errorf("expected body %q, got %q", "flushed content", recorder.Body.String())
		}
	})
}

func TestContactGroupListSerialization(t *testing.T) {
	// Test that contact group lists can be properly serialized and deserialized
	original := notificationv1alpha1.ContactGroupList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ContactGroupList",
			APIVersion: "notification.miloapis.com/v1alpha1",
		},
		Items: []notificationv1alpha1.ContactGroup{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "group1", Namespace: "default"},
				Spec: notificationv1alpha1.ContactGroupSpec{
					DisplayName: "Test Group",
					Visibility:  notificationv1alpha1.ContactGroupVisibilityPublic,
				},
			},
		},
	}

	// Serialize
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Deserialize
	var restored notificationv1alpha1.ContactGroupList
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(restored.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(restored.Items))
	}
	if restored.Items[0].Name != "group1" {
		t.Errorf("expected name %q, got %q", "group1", restored.Items[0].Name)
	}
}

// TestContactGroupVisibilityFilter_Visibility tests the visibility logic with a mock client
func TestContactGroupVisibilityFilter_Visibility(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = notificationv1alpha1.AddToScheme(scheme)

	tests := []struct {
		name              string
		userID            string
		contact           *notificationv1alpha1.Contact
		memberships       []notificationv1alpha1.ContactGroupMembership
		removals          []notificationv1alpha1.ContactGroupMembershipRemoval
		inputGroups       []notificationv1alpha1.ContactGroup
		expectedGroupKeys []string
	}{
		{
			name:   "public groups are always visible",
			userID: "user-1",
			contact: &notificationv1alpha1.Contact{
				ObjectMeta: metav1.ObjectMeta{Name: "contact-1", Namespace: "default"},
				Spec:       notificationv1alpha1.ContactSpec{SubjectRef: &notificationv1alpha1.SubjectReference{Name: "user-1"}},
			},
			inputGroups: []notificationv1alpha1.ContactGroup{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "public-1", Namespace: "default"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPublic},
				},
			},
			expectedGroupKeys: []string{"default/public-1"},
		},
		{
			name:   "private group visible with membership",
			userID: "user-1",
			contact: &notificationv1alpha1.Contact{
				ObjectMeta: metav1.ObjectMeta{Name: "contact-1", Namespace: "user-ns"},
				Spec:       notificationv1alpha1.ContactSpec{SubjectRef: &notificationv1alpha1.SubjectReference{Name: "user-1"}},
			},
			memberships: []notificationv1alpha1.ContactGroupMembership{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "mem-1", Namespace: "user-ns"},
					Spec: notificationv1alpha1.ContactGroupMembershipSpec{
						ContactRef:      notificationv1alpha1.ContactReference{Name: "contact-1"},
						ContactGroupRef: notificationv1alpha1.ContactGroupReference{Name: "private-1", Namespace: "group-ns"},
					},
				},
			},
			inputGroups: []notificationv1alpha1.ContactGroup{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "private-1", Namespace: "group-ns"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPrivate},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "private-2", Namespace: "group-ns"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPrivate},
				},
			},
			expectedGroupKeys: []string{"group-ns/private-1"},
		},
		{
			name:   "private group not visible with removal",
			userID: "user-1",
			contact: &notificationv1alpha1.Contact{
				ObjectMeta: metav1.ObjectMeta{Name: "contact-1", Namespace: "user-ns"},
				Spec:       notificationv1alpha1.ContactSpec{SubjectRef: &notificationv1alpha1.SubjectReference{Name: "user-1"}},
			},
			removals: []notificationv1alpha1.ContactGroupMembershipRemoval{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "rem-1", Namespace: "user-ns"},
					Spec: notificationv1alpha1.ContactGroupMembershipRemovalSpec{
						ContactRef:      notificationv1alpha1.ContactReference{Name: "contact-1"},
						ContactGroupRef: notificationv1alpha1.ContactGroupReference{Name: "private-1", Namespace: "group-ns"},
					},
				},
			},
			inputGroups: []notificationv1alpha1.ContactGroup{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "private-1", Namespace: "group-ns"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPrivate},
				},
			},
			expectedGroupKeys: []string{},
		},
		{
			name:   "private group visible with membership in different namespace",
			userID: "user-1",
			contact: &notificationv1alpha1.Contact{
				ObjectMeta: metav1.ObjectMeta{Name: "contact-1", Namespace: "user-ns"},
				Spec:       notificationv1alpha1.ContactSpec{SubjectRef: &notificationv1alpha1.SubjectReference{Name: "user-1"}},
			},
			memberships: []notificationv1alpha1.ContactGroupMembership{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "mem-1", Namespace: "other-ns"},
					Spec: notificationv1alpha1.ContactGroupMembershipSpec{
						ContactRef:      notificationv1alpha1.ContactReference{Name: "contact-1", Namespace: "user-ns"},
						ContactGroupRef: notificationv1alpha1.ContactGroupReference{Name: "private-1", Namespace: "group-ns"},
					},
				},
			},
			inputGroups: []notificationv1alpha1.ContactGroup{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "private-1", Namespace: "group-ns"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPrivate},
				},
			},
			expectedGroupKeys: []string{"group-ns/private-1"},
		},
		{
			name:   "no contact found keeps only public groups",
			userID: "user-unknown",
			inputGroups: []notificationv1alpha1.ContactGroup{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "public-1", Namespace: "default"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPublic},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "private-1", Namespace: "default"},
					Spec:       notificationv1alpha1.ContactGroupSpec{Visibility: notificationv1alpha1.ContactGroupVisibilityPrivate},
				},
			},
			expectedGroupKeys: []string{"default/public-1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup fake client
			f := NewContactGroupVisibilityFilter(nil)
			f.clientGetter = func() (dynamic.Interface, error) {
				// Convert typed objects to unstructured for the dynamic client
				scheme := runtime.NewScheme()
				_ = notificationv1alpha1.AddToScheme(scheme)

				var objects []runtime.Object
				if tc.contact != nil {
					objects = append(objects, tc.contact)
				}
				for _, m := range tc.memberships {
					obj := m // copy
					objects = append(objects, &obj)
				}
				for _, r := range tc.removals {
					obj := r // copy
					objects = append(objects, &obj)
				}

				return fake.NewSimpleDynamicClient(scheme, objects...), nil
			}

			// Prepare input body
			inputList := notificationv1alpha1.ContactGroupList{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ContactGroupList",
					APIVersion: "notification.miloapis.com/v1alpha1",
				},
				Items: tc.inputGroups,
			}
			inputBody, _ := json.Marshal(inputList)

			// Execute filter
			outputBody, err := f.filterContactGroups(context.Background(), tc.userID, inputBody)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify output
			var outputList notificationv1alpha1.ContactGroupList
			if err := json.Unmarshal(outputBody, &outputList); err != nil {
				t.Fatalf("failed to unmarshal output: %v", err)
			}

			if len(outputList.Items) != len(tc.expectedGroupKeys) {
				t.Errorf("expected %d groups, got %d", len(tc.expectedGroupKeys), len(outputList.Items))
			}

			// Check each expected group is present
			for _, expectedKey := range tc.expectedGroupKeys {
				found := false
				for _, group := range outputList.Items {
					key := fmt.Sprintf("%s/%s", group.Namespace, group.Name)
					if key == expectedKey {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected group %s not found in output", expectedKey)
				}
			}
		})
	}
}
