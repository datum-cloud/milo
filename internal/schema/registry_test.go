package schema_test

import (
	"context"
	"testing"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"go.datum.net/iam/internal/schema"
	"go.datum.net/iam/internal/storage"
)

func TestResourceRegistry(t *testing.T) {
	services := &storage.InMemory[*iampb.Service]{}
	registry := schema.Registry{
		Services: services,
	}

	_, err := services.CreateResource(context.Background(), &storage.CreateResourceRequest[*iampb.Service]{
		Name: "services/library.example.com",
		Resource: &iampb.Service{
			Name:      "services/library.example.com",
			ServiceId: "library.example.com",
			Spec: &iampb.ServiceSpec{
				Resources: []*iampb.Resource{
					{
						Type:     "library.example.com/Branch",
						Singular: "branch",
						Plural:   "branches",
						ResourceNamePatterns: []string{
							"branches/{branch}",
						},
					},
					{
						Type:     "library.example.com/Book",
						Singular: "book",
						Plural:   "books",
						ResourceNamePatterns: []string{
							"branches/{branch}/books/{book}",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create service: %s", err)
	}

	testCases := []struct {
		fullResourceName     string
		expectedResourceType string
		expectedResourceName string
	}{
		{
			fullResourceName:     "library.example.com/branches/central-park-new-york",
			expectedResourceType: "library.example.com/Branch",
			expectedResourceName: "branches/central-park-new-york",
		},
		{
			fullResourceName:     "library.example.com/branches/central-park-new-york/books/alice-in-wonderland",
			expectedResourceType: "library.example.com/Book",
			expectedResourceName: "branches/central-park-new-york/books/alice-in-wonderland",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.fullResourceName, func(t *testing.T) {
			reference, err := registry.ResolveResource(context.Background(), testCase.fullResourceName)
			if err != nil {
				t.Errorf("failed to resolve resource reference: %s", err)
				return
			}
			if reference.Type != testCase.expectedResourceType {
				t.Errorf("expected name to resolve to resource type '%s' but got '%s", testCase.expectedResourceType, reference.Type)
			}
			if reference.Name != testCase.expectedResourceName {
				t.Errorf("expected name to resolve to resource name '%s' but got '%s", testCase.expectedResourceName, reference.Name)
			}
		})
	}
}
