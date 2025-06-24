// Copyright 2024 The Milo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filters

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"

	"go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
)

func TestWithOrganizationMembership(t *testing.T) {
	testCases := []struct {
		name                   string
		fieldSelector          string
		expectedParentName     string
		expectedParentType     string
		expectedParentAPIGroup string
	}{
		{
			name:                   "user field selector",
			fieldSelector:          "spec.userRef.name=test-user",
			expectedParentName:     "test-user",
			expectedParentType:     "User",
			expectedParentAPIGroup: "iam.miloapis.com",
		},
		{
			name:                   "organization field selector",
			fieldSelector:          "spec.organizationRef.name=test-org",
			expectedParentName:     "test-org",
			expectedParentType:     "Organization",
			expectedParentAPIGroup: "resourcemanager.miloapis.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := WithOrganizationMembership(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				u, ok := request.UserFrom(req.Context())
				if !ok {
					t.Fatal("user not found in context")
				}
				extra := u.GetExtra()
				if extra == nil {
					t.Fatal("extra not found on user")
				}
				if extra[v1alpha1.ParentNameExtraKey][0] != tc.expectedParentName {
					t.Fatalf("expected extra key %q to be %q, got %q", v1alpha1.ParentNameExtraKey, tc.expectedParentName, extra[v1alpha1.ParentNameExtraKey][0])
				}
				if extra[v1alpha1.ParentKindExtraKey][0] != tc.expectedParentType {
					t.Fatalf("expected extra key %q to be %q, got %q", v1alpha1.ParentKindExtraKey, tc.expectedParentType, extra[v1alpha1.ParentKindExtraKey][0])
				}
				if extra[v1alpha1.ParentAPIGroupExtraKey][0] != tc.expectedParentAPIGroup {
					t.Fatalf("expected extra key %q to be %q, got %q", v1alpha1.ParentAPIGroupExtraKey, tc.expectedParentAPIGroup, extra[v1alpha1.ParentAPIGroupExtraKey][0])
				}
			}))

			req := httptest.NewRequest("GET", "http://localhost?fieldSelector="+tc.fieldSelector, nil)
			ctx := request.WithRequestInfo(req.Context(), &request.RequestInfo{
				IsResourceRequest: true,
				APIGroup:          "iam.miloapis.com",
				Resource:          "organizationmemberships",
			})
			ctx = request.WithUser(ctx, &user.DefaultInfo{
				Name: "test-user",
			})
			req = req.WithContext(ctx)

			handler.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
}
