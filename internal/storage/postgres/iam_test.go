package postgres_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/lib/pq"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"github.com/google/uuid"
	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/storage/postgres"
)

func TestIAMServiceStorage(t *testing.T) {
	t.Skip("Disabling the IAM service storage testing until we can fix having it run in a stable way")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := sql.Open("postgres", "postgres://postgres:password@localhost:5432/datum?sslmode=disable")
	if err != nil {
		t.Fatalf("failed to open new database connection: %s", err)
	}

	serviceStorage, err := postgres.ResourceServer(db, &iampb.Service{})
	if err != nil {
		t.Fatalf("failed to create new resource server: %s", err)
	}

	service, err := serviceStorage.CreateResource(ctx, &storage.CreateResourceRequest[*iampb.Service]{
		Name: "services/compute.datumapis.com",
		Resource: &iampb.Service{
			ServiceId:   "compute.datumapis.com",
			Name:        "services/compute.datumapis.com",
			Uid:         uuid.NewString(),
			DisplayName: "Datum Cloud Compute",
			Spec: &iampb.ServiceSpec{
				Resources: []*iampb.Resource{{
					Type:     "compute.datumapis.com/Workload",
					Singular: "workload",
					Plural:   "workloads",
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create service: %s", err)
	}

	if service.Uid == "" {
		t.Error("UID was not expected to be empty")
	}

	list, err := serviceStorage.ListResources(ctx, &storage.ListResourcesRequest{
		Parent: "testing",
	})
	if err != nil {
		t.Fatalf("failed to list resource: %s", err)
	}

	if len(list.Resources) == 0 {
		t.Error("failed to confirm a list of resources were returned")
	}
}
