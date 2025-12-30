// Copyright 2024 The Milo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
)

// ContactGroupVisibilityFilter provides visibility filtering for ContactGroups.
// It ensures that:
//   - Public contact groups are visible to all users
//   - Private contact groups are only visible to users who have an associated
//     ContactGroupMembership
type ContactGroupVisibilityFilter struct {
	loopbackConfig *rest.Config
	clientGetter   func() (dynamic.Interface, error)
}

// NewContactGroupVisibilityFilter creates a new ContactGroupVisibilityFilter.
func NewContactGroupVisibilityFilter(loopbackConfig *rest.Config) *ContactGroupVisibilityFilter {
	return &ContactGroupVisibilityFilter{
		loopbackConfig: loopbackConfig,
		clientGetter: func() (dynamic.Interface, error) {
			return dynamic.NewForConfig(loopbackConfig)
		},
	}
}

// ContactGroupVisibilityDecorator intercepts list/get requests for contactgroups
// and filters out private groups that the user doesn't have access to.
//
// Visibility rules:
//   - Public groups: visible to everyone
//   - Private groups: only visible if the user has a ContactGroupMembership associated with that group
func ContactGroupVisibilityDecorator(loopbackConfig *rest.Config) func(http.Handler) http.Handler {
	filter := NewContactGroupVisibilityFilter(loopbackConfig)
	return filter.Wrap
}

// Wrap wraps the provided handler with contact group visibility filtering.
func (f *ContactGroupVisibilityFilter) Wrap(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		info, ok := request.RequestInfoFrom(ctx)
		if !ok {
			responsewriters.InternalError(w, req, fmt.Errorf("failed to get RequestInfo from context"))
			return
		}

		// Only intercept contactgroups list requests
		if info.APIGroup != notificationv1alpha1.SchemeGroupVersion.Group ||
			info.Resource != "contactgroups" ||
			info.Verb != "list" {
			handler.ServeHTTP(w, req)
			return
		}

		// Get the user ID from the context (set by UserContextHandler)
		userID, ok := ctx.Value(UserIDContextKey).(string)
		if !ok {
			// Not a user-scoped request, pass through
			handler.ServeHTTP(w, req)
			return
		}

		// Use a custom response writer to capture the response
		captureWriter := newCaptureResponseWriter(w)
		handler.ServeHTTP(captureWriter, req)

		// If the upstream handler didn't succeed, just forward the response
		if captureWriter.statusCode != http.StatusOK {
			captureWriter.flush()
			return
		}

		// Filter the contact groups based on visibility
		filteredBody, err := f.filterContactGroups(ctx, userID, captureWriter.body)
		if err != nil {
			responsewriters.InternalError(w, req, fmt.Errorf("failed to filter contact groups: %w", err))
			return
		}

		// Write the filtered response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(filteredBody)
	})
}

// filterContactGroups filters the list of contact groups based on visibility rules.
// Supports both ContactGroupList/List responses and Table responses (kubectl default).
func (f *ContactGroupVisibilityFilter) filterContactGroups(ctx context.Context, userID string, body []byte) ([]byte, error) {
	// First, check the response kind to determine how to filter it.
	var typeMeta struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(body, &typeMeta); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response type: %w", err)
	}

	// Get dynamic client for lookups
	client, err := f.clientGetter()
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamic client: %w", err)
	}

	// Build the set of accessible groups for this user
	accessibleGroups, hasContact := f.buildAccessibleGroupsSet(ctx, client, userID)

	switch typeMeta.Kind {
	case "Table":
		return f.filterTableResponse(ctx, client, body, accessibleGroups, hasContact)
	case "ContactGroupList", "List":
		return f.filterListResponse(body, accessibleGroups, hasContact)
	default:
		// Unknown format, pass through unmodified
		return body, nil
	}
}

// buildAccessibleGroupsSet returns the set of group keys (namespace/name) accessible to the user.
// Returns the map and a boolean indicating if the user has an associated Contact.
func (f *ContactGroupVisibilityFilter) buildAccessibleGroupsSet(ctx context.Context, client dynamic.Interface, userID string) (map[string]bool, bool) {
	contactName, contactNamespace, err := f.findContactForUser(ctx, client, userID)
	if err != nil {
		// No contact found, user can only see public groups
		return nil, false
	}

	accessibleGroups, err := f.getAccessibleGroups(ctx, client, contactName, contactNamespace)
	if err != nil {
		// On error, fall back to showing only public groups
		return nil, true
	}

	return accessibleGroups, true
}

// filterListResponse filters a ContactGroupList or List response.
func (f *ContactGroupVisibilityFilter) filterListResponse(body []byte, accessibleGroups map[string]bool, hasContact bool) ([]byte, error) {
	var contactGroupList notificationv1alpha1.ContactGroupList
	if err := json.Unmarshal(body, &contactGroupList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contact group list: %w", err)
	}

	filteredGroups := f.filterGroupItems(contactGroupList.Items, accessibleGroups, hasContact)
	contactGroupList.Items = filteredGroups
	return json.Marshal(contactGroupList)
}

// filterTableResponse filters a Table response (kubectl default format).
func (f *ContactGroupVisibilityFilter) filterTableResponse(ctx context.Context, client dynamic.Interface, body []byte, accessibleGroups map[string]bool, hasContact bool) ([]byte, error) {
	var table metav1.Table
	if err := json.Unmarshal(body, &table); err != nil {
		return nil, fmt.Errorf("failed to unmarshal table: %w", err)
	}

	// Filter table rows based on the embedded object metadata
	filteredRows := make([]metav1.TableRow, 0, len(table.Rows))
	contactGroupGVR := notificationv1alpha1.SchemeGroupVersion.WithResource("contactgroups")

	for _, row := range table.Rows {
		// Each row has an Object field containing PartialObjectMetadata
		if row.Object.Raw == nil {
			continue
		}

		// Parse the partial object metadata to get name and namespace
		var partialMeta metav1.PartialObjectMetadata
		if err := json.Unmarshal(row.Object.Raw, &partialMeta); err != nil {
			// Skip rows we can't parse
			continue
		}

		// Fetch the full ContactGroup to check visibility
		unstructGroup, err := client.Resource(contactGroupGVR).Namespace(partialMeta.Namespace).Get(ctx, partialMeta.Name, metav1.GetOptions{})
		if err != nil {
			// Skip groups we can't fetch
			continue
		}

		var group notificationv1alpha1.ContactGroup
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructGroup.Object, &group); err != nil {
			continue
		}

		// Apply visibility filter
		if f.isGroupVisible(group, accessibleGroups, hasContact) {
			filteredRows = append(filteredRows, row)
		}
	}

	table.Rows = filteredRows
	return json.Marshal(table)
}

// filterGroupItems filters a slice of ContactGroup items based on visibility rules.
func (f *ContactGroupVisibilityFilter) filterGroupItems(groups []notificationv1alpha1.ContactGroup, accessibleGroups map[string]bool, hasContact bool) []notificationv1alpha1.ContactGroup {
	filtered := make([]notificationv1alpha1.ContactGroup, 0, len(groups))
	for _, group := range groups {
		if f.isGroupVisible(group, accessibleGroups, hasContact) {
			filtered = append(filtered, group)
		}
	}
	return filtered
}

// isGroupVisible determines if a contact group is visible to the user.
func (f *ContactGroupVisibilityFilter) isGroupVisible(group notificationv1alpha1.ContactGroup, accessibleGroups map[string]bool, hasContact bool) bool {
	// Public groups are always visible
	if group.Spec.Visibility == notificationv1alpha1.ContactGroupVisibilityPublic {
		return true
	}

	// Private groups require membership
	if accessibleGroups == nil {
		return false
	}

	groupKey := fmt.Sprintf("%s/%s", group.Namespace, group.Name)
	return accessibleGroups[groupKey]
}

// findContactForUser finds the Contact resource for the given user ID.
// Searches across all namespaces and returns the contact name, namespace, and any error.
func (f *ContactGroupVisibilityFilter) findContactForUser(ctx context.Context, client dynamic.Interface, userID string) (string, string, error) {
	// Query contacts across all namespaces
	contactGVR := notificationv1alpha1.SchemeGroupVersion.WithResource("contacts")

	// List all contacts and find one with matching subject reference
	contactList, err := client.Resource(contactGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to list contacts: %w", err)
	}

	for _, unstructContact := range contactList.Items {
		var contact notificationv1alpha1.Contact
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructContact.Object, &contact); err != nil {
			// Skip objects that don't match our schema
			continue
		}

		if contact.Spec.SubjectRef != nil && contact.Spec.SubjectRef.Name == userID {
			return contact.Name, contact.Namespace, nil
		}
	}

	return "", "", errors.NewNotFound(contactGVR.GroupResource(), userID)
}

// getAccessibleGroups returns the list of contact group keys (namespace/name) that the user has access to
// (through membership or removal records). Searches across all namespaces.
func (f *ContactGroupVisibilityFilter) getAccessibleGroups(ctx context.Context, client dynamic.Interface, contactName, contactNamespace string) (map[string]bool, error) {
	accessibleGroups := make(map[string]bool)

	// Get memberships for this contact across all namespaces
	membershipGVR := notificationv1alpha1.SchemeGroupVersion.WithResource("contactgroupmemberships")
	membershipList, err := client.Resource(membershipGVR).List(ctx, metav1.ListOptions{
		// Note: Field selector filters on contactRef.name across all namespaces
		FieldSelector: fmt.Sprintf("spec.contactRef.name=%s", contactName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list memberships: %w", err)
	}

	for _, unstructMembership := range membershipList.Items {
		var membership notificationv1alpha1.ContactGroupMembership
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructMembership.Object, &membership); err != nil {
			continue
		}

		// Verify this membership is for our contact (matching name and namespace)
		refNamespace := membership.Spec.ContactRef.Namespace
		if refNamespace == "" {
			refNamespace = membership.Namespace
		}
		if membership.Spec.ContactRef.Name != contactName || refNamespace != contactNamespace {
			continue
		}

		groupName := membership.Spec.ContactGroupRef.Name
		groupNamespace := membership.Spec.ContactGroupRef.Namespace
		if groupNamespace == "" {
			groupNamespace = membership.Namespace
		}
		accessibleGroups[fmt.Sprintf("%s/%s", groupNamespace, groupName)] = true
	}

	return accessibleGroups, nil
}

// captureResponseWriter captures the response for post-processing.
type captureResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       []byte
}

func newCaptureResponseWriter(w http.ResponseWriter) *captureResponseWriter {
	return &captureResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (c *captureResponseWriter) WriteHeader(code int) {
	c.statusCode = code
}

func (c *captureResponseWriter) Write(b []byte) (int, error) {
	c.body = append(c.body, b...)
	return len(b), nil
}

func (c *captureResponseWriter) flush() {
	c.ResponseWriter.WriteHeader(c.statusCode)
	_, _ = c.ResponseWriter.Write(c.body)
}
