// Copyright 2024 The Milo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filters

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
)

const (
	// OrganizationMembershipUserFieldSelector is the field selector for the user in an organization membership.
	OrganizationMembershipUserFieldSelector = "spec.userRef.name"
	// OrganizationMembershipOrganizationFieldSelector is the field selector for the organization in an organization membership.
	OrganizationMembershipOrganizationFieldSelector = "spec.organizationRef.name"
)

const (
	ParentAPIGroupContextKey = "parentAPIGroup"
	ParentKindContextKey     = "parentKind"
	ParentNameContextKey     = "parentName"
)

// OrganizationMembershipContextHandler is a filter that inspects requests for OrganizationMembership resources
// and augments the user's authentication information with the values from the field selectors.
func OrganizationMembershipContextHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		log := logf.FromContext(ctx)

		info, ok := request.RequestInfoFrom(ctx)
		if !ok {
			log.Error(nil, "request info not found in context")
			// if this happens, the request info resolver is missing from the chain
			handler.ServeHTTP(w, req)
			return
		}

		log = log.WithValues("request_info", info)
		log.Info("processing organization membership request")

		if !info.IsResourceRequest {
			log.Info("not a resource request")
			handler.ServeHTTP(w, req)
			return
		}

		if info.APIGroup != "iam.miloapis.com" || info.Resource != "organizationmemberships" {
			log.Info("not an organization membership request")
			handler.ServeHTTP(w, req)
			return
		}

		fieldSelector := req.URL.Query().Get("fieldSelector")
		if fieldSelector == "" {
			log.Info("no field selector provided")
			handler.ServeHTTP(w, req)
			return
		}

		selector, err := fields.ParseSelector(fieldSelector)
		if err != nil {
			log.Error(err, "invalid field selector")
			// malformed field selector, let the validation handle it
			handler.ServeHTTP(w, req)
			return
		}

		requirements := selector.Requirements()
		if len(requirements) != 1 {
			log.Info("multiple field selectors provided")
			handler.ServeHTTP(w, req)
			return
		}

		for _, requirement := range requirements {
			switch requirement.Field {
			case OrganizationMembershipUserFieldSelector:
				ctx = context.WithValue(ctx, ParentAPIGroupContextKey, v1alpha1.SchemeGroupVersion.Group)
				ctx = context.WithValue(ctx, ParentKindContextKey, "User")
				ctx = context.WithValue(ctx, ParentNameContextKey, requirement.Value)

			case OrganizationMembershipOrganizationFieldSelector:
				ctx = context.WithValue(ctx, ParentAPIGroupContextKey, resourcemanagerv1alpha1.GroupVersion.Group)
				ctx = context.WithValue(ctx, ParentKindContextKey, "Organization")
				ctx = context.WithValue(ctx, ParentNameContextKey, requirement.Value)
			}
			log.Info("added parent info to context", "parent_api_group", ctx.Value(ParentAPIGroupContextKey), "parent_kind", ctx.Value(ParentKindContextKey), "parent_name", ctx.Value(ParentNameContextKey))
		}

		req = req.WithContext(ctx)
		handler.ServeHTTP(w, req)
	})
}

// OrganizationContextAuthorizationDecorator needs to run after authentication,
// but prior to authorization.
//
// This handler injects organization information into the authenticated user's
// Extra information that's made available in the request context by
// the `organizationContextHandler` handler.
func OrganizationMembershipContextAuthorizationDecorator(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		log := logf.FromContext(ctx)

		info, ok := request.RequestInfoFrom(ctx)
		if !ok {
			log.Error(nil, "request info not found in context")
			// if this happens, the request info resolver is missing from the chain
			handler.ServeHTTP(w, req)
			return
		}

		if !info.IsResourceRequest {
			log.Info("not a resource request")
			handler.ServeHTTP(w, req)
			return
		}

		log = log.WithValues("request_info", info)
		log.Info("processing organization membership authorization")

		parentAPIGroup, parentAPIGroupOk := ctx.Value(ParentAPIGroupContextKey).(string)
		parentKind, parentKindOk := ctx.Value(ParentKindContextKey).(string)
		parentName, parentNameOk := ctx.Value(ParentNameContextKey).(string)

		if !parentAPIGroupOk || !parentKindOk || !parentNameOk {
			// Not an org scoped request
			log.Info("not an org scoped request")
			handler.ServeHTTP(w, req)
			return
		}

		reqUser, ok := request.UserFrom(ctx)
		if !ok {
			log.Error(nil, "failed to extract user info from context")
			// error handling
			responsewriters.InternalError(w, req, fmt.Errorf("failed to extract user info from context"))
			return
		}

		u, ok := reqUser.(*user.DefaultInfo)
		if !ok {
			log.Error(nil, "unexpected user.Info type", "user_info_type", fmt.Sprintf("%T", reqUser))
			responsewriters.InternalError(w, req, fmt.Errorf("unexpected user.Info type. Expected *user.DefaultInfo, got %T", reqUser))
			return
		}

		if u.Extra == nil {
			u.Extra = map[string][]string{}
		}

		// Set the parent resource information for the authorization check based on
		// the organization ID that was provided in the request context.
		u.Extra[iamv1alpha1.ParentAPIGroupExtraKey] = []string{parentAPIGroup}
		u.Extra[iamv1alpha1.ParentKindExtraKey] = []string{parentKind}
		u.Extra[iamv1alpha1.ParentNameExtraKey] = []string{parentName}

		req = req.WithContext(request.WithUser(ctx, u))

		handler.ServeHTTP(w, req)
	})
}

func augmentUser(u user.Info, selector fields.Selector) user.Info {
	extra := u.GetExtra()
	if extra == nil {
		extra = make(map[string][]string)
	}

	return &user.DefaultInfo{
		Name:   u.GetName(),
		UID:    u.GetUID(),
		Groups: u.GetGroups(),
		Extra:  extra,
	}
}
