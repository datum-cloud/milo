// Integration testing for the IAM service.
package server_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"

	"buf.build/gen/go/datum-cloud/iam/grpc/go/datum/iam/v1alpha/iamv1alphagrpc"
	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/compose"
	"go.datum.net/iam/internal/grpc/logging"
	"go.datum.net/iam/internal/grpc/server"
	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/subject"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestIAMEndToEnd(t *testing.T) {
	env, err := compose.NewDockerComposeWith(
		compose.WithLogger(testcontainers.TestLogger(t)),
		compose.WithStackFiles("../../../test/docker-compose.yaml"),
	)
	if err != nil {
		t.Fatalf("failed to create environment: %s", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := env.Up(ctx, compose.Wait(true)); err != nil {
		t.Fatalf("failed to start environment: %s", err)
	}
	defer env.Down(ctx)

	// Creates a new gRPC client we can use to make API requests to a backend
	// that implements Datum's Identity and Access Management (IAM) API.
	clients := setupIAMClient(t, ctx)
	defer clients.Close()

	// Services will be registered with the IAM system to help configure the
	// necessary relationships needed to check a user's access to resources on
	// the platform.
	//
	// Service will register IAM permissions, the resources they manage, and any
	// default roles they want to offer to consumers. Consumers will be able to
	// build their own custom roles if they would like.
	err = clients.RegisterService(ctx, &ServiceRegistration{
		Service: &iampb.Service{
			ServiceId:   "a-really-long-library-name.example.api",
			DisplayName: "Example Library API",
			Spec: &iampb.ServiceSpec{
				Resources: []*iampb.Resource{
					{
						Type:     "a-really-long-library-name.example.api/Branch",
						Singular: "branch",
						Plural:   "branches",
						ResourceNamePatterns: []string{
							"branches/{branch}",
						},
						Permissions: []string{
							"list",
							"get",
							"create",
							"update",
							"delete",
						},
					},
					{
						Type:     "a-really-long-library-name.example.api/Book",
						Singular: "book",
						Plural:   "books",
						ParentResources: []string{
							"a-really-long-library-name.example.api/Branch",
						},
						ResourceNamePatterns: []string{
							"branches/{branch}/books/{book}",
						},
						Permissions: []string{
							"list",
							"get",
							"create",
							"update",
							"delete",
							"checkout",
							"return",
						},
					},
				},
			},
		},
		Roles: []*iampb.Role{
			{
				Name:        "services/a-really-long-library-name.example.api/roles/libraryAdmin",
				RoleId:      "libraryAdmin",
				DisplayName: "Library Admin",
				Spec: &iampb.RoleSpec{
					IncludedPermissions: []string{
						"a-really-long-library-name.example.api/books.list",
						"a-really-long-library-name.example.api/books.get",
						"a-really-long-library-name.example.api/books.create",
						"a-really-long-library-name.example.api/books.update",
						"a-really-long-library-name.example.api/books.delete",
						"a-really-long-library-name.example.api/branches.list",
						"a-really-long-library-name.example.api/branches.get",
						"a-really-long-library-name.example.api/branches.create",
						"a-really-long-library-name.example.api/branches.update",
						"a-really-long-library-name.example.api/branches.delete",
					},
				},
			},
			{
				Name:        "services/a-really-long-library-name.example.api/roles/bookRenter",
				RoleId:      "bookRenter",
				DisplayName: "Book Renter",
				Spec: &iampb.RoleSpec{
					IncludedPermissions: []string{
						"a-really-long-library-name.example.api/branches.list",
						"a-really-long-library-name.example.api/branches.get",
						"a-really-long-library-name.example.api/books.list",
						"a-really-long-library-name.example.api/books.get",
						"a-really-long-library-name.example.api/books.checkout",
						"a-really-long-library-name.example.api/books.return",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to register library service: %s", err)
	}

	// Setup some library branches and books we can use to check access in the
	// system.
	err = setupBranches(clients, ctx, []*LibraryBranch{
		{
			Name: "central-park-new-york",
			IAMPolicy: &iampb.Policy{
				Spec: &iampb.PolicySpec{
					Bindings: []*iampb.Binding{{
						Role:    "services/a-really-long-library-name.example.api/roles/libraryAdmin",
						Members: []string{"user:branch-admin@new-york.libraries"},
					}},
				},
			},
			Books: []*LibraryBook{
				{
					Name: "alice-in-wonderland",
					IAMPolicy: &iampb.Policy{
						Spec: &iampb.PolicySpec{
							Bindings: []*iampb.Binding{{
								Role:    "services/a-really-long-library-name.example.api/roles/bookRenter",
								Members: []string{"user:book-renter@example.com"},
							}},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to setup library branches: %s", err)
	}

	// Configure a set of access checks to run against the system for various
	// resources.
	accessChecks := []TestAccessCheck{
		{
			Subject: "user:branch-admin@new-york.libraries",
			ResourceChecks: []ResourceCheck{
				//
				{
					Resource: "a-really-long-library-name.example.api/branches/central-park-new-york",
					Permissions: []PermissionCheck{
						{Name: "a-really-long-library-name.example.api/branches.create", Allowed: true},
						{Name: "a-really-long-library-name.example.api/branches.delete", Allowed: true},
					},
				},
				{
					Resource: "a-really-long-library-name.example.api/branches/central-park-new-york/books/alice-in-wonderland",
					Permissions: []PermissionCheck{
						{Name: "a-really-long-library-name.example.api/books.create", Allowed: true},
						{Name: "a-really-long-library-name.example.api/books.delete", Allowed: true},
					},
					Context: []*iampb.CheckContext{{
						ContextType: &iampb.CheckContext_ParentRelationship{
							ParentRelationship: &iampb.ParentRelationship{
								ParentResource: "a-really-long-library-name.example.api/branches/central-park-new-york",
								ChildResource:  "a-really-long-library-name.example.api/branches/central-park-new-york/books/alice-in-wonderland",
							},
						},
					}},
				},
			},
		},
		{
			Subject: "user:book-renter@example.com",
			ResourceChecks: []ResourceCheck{
				//
				{
					Resource: "a-really-long-library-name.example.api/branches/central-park-new-york",
					Permissions: []PermissionCheck{
						{Name: "a-really-long-library-name.example.api/branches.create", Allowed: false},
						{Name: "a-really-long-library-name.example.api/branches.delete", Allowed: false},
					},
				},
				{
					Resource: "a-really-long-library-name.example.api/branches/central-park-new-york/books/alice-in-wonderland",
					Permissions: []PermissionCheck{
						{Name: "a-really-long-library-name.example.api/books.checkout", Allowed: true},
						{Name: "a-really-long-library-name.example.api/books.return", Allowed: true},
					},
					Context: []*iampb.CheckContext{{
						ContextType: &iampb.CheckContext_ParentRelationship{
							ParentRelationship: &iampb.ParentRelationship{
								ParentResource: "a-really-long-library-name.example.api/branches/central-park-new-york",
								ChildResource:  "a-really-long-library-name.example.api/branches/central-park-new-york/books/alice-in-wonderland",
							},
						},
					}},
				},
			},
		},
	}

	for _, accessCheck := range accessChecks {
		t.Run(accessCheck.Subject, func(t *testing.T) {
			for _, resourceCheck := range accessCheck.ResourceChecks {
				t.Run(strings.ReplaceAll(resourceCheck.Resource, "/", ":"), func(t *testing.T) {
					for _, permissionCheck := range resourceCheck.Permissions {
						t.Run(strings.ReplaceAll(permissionCheck.Name, "/", ":"), func(t *testing.T) {
							request := &iampb.CheckAccessRequest{
								Subject:    accessCheck.Subject,
								Permission: permissionCheck.Name,
								Resource:   resourceCheck.Resource,
								Context:    resourceCheck.Context,
							}

							if resp, err := clients.Access.CheckAccess(ctx, request); err != nil {
								t.Fatalf("failed to check user access: %s", err)
							} else if permissionCheck.Allowed && !resp.Allowed {
								t.Errorf("Expected `%s` to have `%s` on `%s`", accessCheck.Subject, permissionCheck.Name, resourceCheck.Resource)
							} else if !permissionCheck.Allowed && resp.Allowed {
								t.Errorf("Expected `%s` to NOT have `%s` on `%s`", accessCheck.Subject, permissionCheck.Name, resourceCheck.Resource)
							}
						})
					}
				})
			}
		})
	}
}

func (c *Client) RegisterService(ctx context.Context, service *ServiceRegistration) error {
	// Creates a new service in the IAM system with the permissions that the
	// service is expecting to check.
	createServiceOperation, err := c.Services.CreateService(ctx, &iampb.CreateServiceRequest{
		ServiceId: service.Service.ServiceId,
		Service:   service.Service,
	})
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	createdService := &iampb.Service{}
	if err := anypb.UnmarshalTo(createServiceOperation.GetResponse(), createdService, proto.UnmarshalOptions{}); err != nil {
		return fmt.Errorf("failed to get service from operation: %w", err)
	}

	for _, role := range service.Roles {
		_, err = c.Roles.CreateRole(ctx, &iampb.CreateRoleRequest{
			Parent: createdService.Name,
			RoleId: role.RoleId,
			Role:   role,
		})
		if err != nil {
			return fmt.Errorf("failed to create role '%s': %w", role.RoleId, err)
		}
	}

	return nil
}

func setupBranches(clients *Client, ctx context.Context, branches []*LibraryBranch) error {
	for _, branch := range branches {
		if err := setupBooks(clients, ctx, branch.Name, branch.Books); err != nil {
			return fmt.Errorf("failed to setup library branch's books: %w", err)
		}
		if branch.IAMPolicy == nil {
			return nil
		}

		branch.IAMPolicy.Name = "a-really-long-library-name.example.api/branches/" + branch.Name

		_, err := clients.Policy.SetIamPolicy(ctx, &iampb.SetIamPolicyRequest{
			Policy: branch.IAMPolicy,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func setupBooks(clients *Client, ctx context.Context, branchName string, books []*LibraryBook) error {
	for _, book := range books {
		if book.IAMPolicy != nil {
			book.IAMPolicy.Name = fmt.Sprintf("a-really-long-library-name.example.api/branches/%s/books/%s", branchName, book.Name)

			_, err := clients.Policy.SetIamPolicy(ctx, &iampb.SetIamPolicyRequest{
				Policy: book.IAMPolicy,
			})
			if err != nil {
				return fmt.Errorf("failed to create book's IAM policy: %w", err)
			}
		}
	}
	return nil
}

func setupIAMClient(t *testing.T, ctx context.Context) (client *Client) {
	// Connect to an OpenFGA backend that will be used by the IAM system for
	// checking user authentication.
	openFGAClient, openFGAStoreID, clientCloser := setupOpenFGAClient(t, ctx)

	servicesStorage := &storage.InMemory[*iampb.Service]{}
	rolesStorage := &storage.InMemory[*iampb.Role]{}
	policyStorage := &storage.InMemory[*iampb.Policy]{}

	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		logging.UnaryServerInterceptor(slog.Default()),
	))

	// Create a new in-memory listener we can use for the gRPC server and then
	// create a client to access the gRPC service.
	grpcListener, err := net.Listen("tcp", ":11000")
	if err != nil {
		t.Fatalf("failed to create gRPC listener: %v", err)
	}

	iamClientConn, err := grpc.NewClient(
		":11000",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to create new in-memory client: %s", err)
	}

	if err := server.NewServer(server.ServerOptions{
		OpenFGAClient:   openFGAClient,
		OpenFGAStoreID:  openFGAStoreID,
		ServiceStorage:  servicesStorage,
		RoleStorage:     rolesStorage,
		PolicyStorage:   policyStorage,
		SubjectResolver: subject.NoopResolver(),
		GRPCServer:      grpcServer,
		RoleResolver: func(ctx context.Context, roleName string) error {
			_, err := rolesStorage.GetResource(ctx, &storage.GetResourceRequest{
				Name: roleName,
			})
			return err
		},
	}); err != nil {
		t.Fatalf("failed to create IAM server: %s", err)
	}

	// start the gRPC server
	go func() {
		err := grpcServer.Serve(grpcListener)
		if err != nil {
			panic(err)
		}
	}()

	clients := &Client{
		Services:       iamv1alphagrpc.NewServicesClient(iamClientConn),
		Roles:          iamv1alphagrpc.NewRolesClient(iamClientConn),
		Policy:         iamv1alphagrpc.NewIAMPolicyClient(iamClientConn),
		Access:         iamv1alphagrpc.NewAccessCheckClient(iamClientConn),
		OpenFGA:        openFGAClient,
		OpenFGAStoreID: openFGAStoreID,
		closer:         clientCloser,
	}

	return clients
}

// Configures a new OpenFGA client that can be used for testing. By default,
// this will create a new Store in the OpenFGA server a connection is
// established with.
func setupOpenFGAClient(t *testing.T, ctx context.Context) (client openfgav1.OpenFGAServiceClient, storeID string, closer io.Closer) {
	openfgaClientConn, err := grpc.NewClient(
		// TODO: Support different backend configurations for testing.
		"localhost:8081",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
				t.Logf("%s:request: %s", method, protojson.Format(req.(proto.Message)))
				err := invoker(ctx, method, req, reply, cc, opts...)
				t.Logf("%s:response: %s", method, protojson.Format(reply.(proto.Message)))
				return err
			},
		),
	)
	if err != nil {
		t.Fatalf("failed to create new gRPC client: %s", err)
	}

	openFGAClient := openfgav1.NewOpenFGAServiceClient(openfgaClientConn)

	// Create a new OpenFGA Store we can use
	resp, err := openFGAClient.CreateStore(ctx, &openfgav1.CreateStoreRequest{
		Name: "testing-store",
	})
	if err != nil {
		t.Fatalf("failed to create new FGA Store: %s", err)
	}

	return openFGAClient, resp.Id, openfgaClientConn
}

type TestAccessCheck struct {
	// The subject of the access request. Typically a user, or a service
	// account.
	Subject string
	// A list of resources that should be checked for permissions.
	ResourceChecks []ResourceCheck
}

type ResourceCheck struct {
	// The resource name the access should be checked against.
	Resource string
	// A list of permissions that should be checked for the user against
	// the resource.
	Permissions []PermissionCheck

	Context []*iampb.CheckContext
}

type PermissionCheck struct {
	// The fully qualified name of the permission to check.
	Name string
	// Whether the permission should be allowed given the state of the
	// system that was setup.
	Allowed bool
}

type LibraryBranch struct {
	Name      string
	Books     []*LibraryBook
	IAMPolicy *iampb.Policy
}

type LibraryBook struct {
	Name      string
	IAMPolicy *iampb.Policy
}

type Client struct {
	Services iamv1alphagrpc.ServicesClient
	Roles    iamv1alphagrpc.RolesClient
	Policy   iamv1alphagrpc.IAMPolicyClient
	Access   iamv1alphagrpc.AccessCheckClient

	OpenFGA        openfgav1.OpenFGAServiceClient
	OpenFGAStoreID string
	closer         io.Closer
}

type ServiceRegistration struct {
	Service *iampb.Service
	Roles   []*iampb.Role
}

func (c *Client) Close() error {
	c.OpenFGA.DeleteStore(context.Background(), &openfgav1.DeleteStoreRequest{
		StoreId: c.OpenFGAStoreID,
	})
	return c.closer.Close()
}
