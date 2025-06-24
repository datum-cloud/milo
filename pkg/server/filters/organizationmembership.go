// Copyright 2024 The Milo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filters

import (
	"net/http"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"

	"go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
)

const (
	// OrganizationMembershipUserFieldSelector is the field selector for the user in an organization membership.
	OrganizationMembershipUserFieldSelector = "spec.userRef.name"
	// OrganizationMembershipOrganizationFieldSelector is the field selector for the organization in an organization membership.
	OrganizationMembershipOrganizationFieldSelector = "spec.organizationRef.name"
)

// WithOrganizationMembership is a filter that inspects requests for OrganizationMembership resources
// and augments the user's authentication information with the values from the field selectors.
func WithOrganizationMembership(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		info, ok := request.RequestInfoFrom(ctx)
		if !ok {
			// if this happens, the request info resolver is missing from the chain
			handler.ServeHTTP(w, req)
			return
		}

		if info.APIGroup != "iam.miloapis.com" || info.Resource != "organizationmemberships" {
			handler.ServeHTTP(w, req)
			return
		}

		fieldSelector := req.URL.Query().Get("fieldSelector")
		if fieldSelector == "" {
			handler.ServeHTTP(w, req)
			return
		}

		selector, err := fields.ParseSelector(fieldSelector)
		if err != nil {
			// malformed field selector, let the validation handle it
			handler.ServeHTTP(w, req)
			return
		}

		u, ok := request.UserFrom(ctx)
		if !ok {
			// should not happen
			handler.ServeHTTP(w, req)
			return
		}

		req = req.WithContext(request.WithUser(ctx, augmentUser(u, selector)))
		handler.ServeHTTP(w, req)
	})
}

func augmentUser(u user.Info, selector fields.Selector) user.Info {
	extra := u.GetExtra()
	if extra == nil {
		extra = make(map[string][]string)
	}

	requirements := selector.Requirements()
	for _, requirement := range requirements {
		switch requirement.Field {
		case OrganizationMembershipUserFieldSelector:
			extra[v1alpha1.ParentNameExtraKey] = []string{requirement.Value}
			extra[v1alpha1.ParentKindExtraKey] = []string{"User"}
			extra[v1alpha1.ParentAPIGroupExtraKey] = []string{v1alpha1.SchemeGroupVersion.Group}
		case OrganizationMembershipOrganizationFieldSelector:
			extra[v1alpha1.ParentNameExtraKey] = []string{requirement.Value}
			extra[v1alpha1.ParentKindExtraKey] = []string{"Organization"}
			extra[v1alpha1.ParentAPIGroupExtraKey] = []string{resourcemanagerv1alpha1.GroupVersion.Group}
		}
	}

	return &user.DefaultInfo{
		Name:   u.GetName(),
		UID:    u.GetUID(),
		Groups: u.GetGroups(),
		Extra:  extra,
	}
}
