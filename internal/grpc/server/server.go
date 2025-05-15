package server

import (
	"context"

	"buf.build/gen/go/datum-cloud/iam/grpc-ecosystem/gateway/v2/datum/iam/v1alpha/iamv1alphagateway"
	"buf.build/gen/go/datum-cloud/iam/grpc-ecosystem/gateway/v2/datum/resourcemanager/v1alpha/resourcemanagerv1alphagateway"
	"buf.build/gen/go/datum-cloud/iam/grpc/go/datum/iam/v1alpha/iamv1alphagrpc"
	"buf.build/gen/go/datum-cloud/iam/grpc/go/datum/resourcemanager/v1alpha/resourcemanagerv1alphagrpc"
	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	resourcemanagerpb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/resourcemanager/v1alpha"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"go.datum.net/iam/internal/providers/authentication"
	"go.datum.net/iam/internal/providers/openfga"
	"go.datum.net/iam/internal/role"
	"go.datum.net/iam/internal/schema"
	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/subject"
	"google.golang.org/grpc"
)

type Server struct {
	// Must embed the unimplemented gRPC servers this server implementation is
	// expected to implement.
	iamv1alphagrpc.UnimplementedIAMPolicyServer
	iamv1alphagrpc.UnimplementedRolesServer
	iamv1alphagrpc.UnimplementedServicesServer
	iamv1alphagrpc.UnimplementedAccessCheckServer
	iamv1alphagrpc.UnimplementedUsersServer
	resourcemanagerv1alphagrpc.UnimplementedOrganizationsServer
	resourcemanagerv1alphagrpc.UnimplementedProjectsServer

	PolicyReconciler             *openfga.PolicyReconciler
	RoleReconciler               *openfga.RoleReconciler
	AuthorizationModelReconciler *openfga.AuthorizationModelReconciler
	OpenFGAClient                openfgav1.OpenFGAServiceClient
	OpenFGAStoreID               string
	ServiceStorage               storage.ResourceServer[*iampb.Service]
	RoleStorage                  storage.ResourceServer[*iampb.Role]
	PolicyStorage                storage.ResourceServer[*iampb.Policy]
	UserStorage                  storage.ResourceServer[*iampb.User]
	OrganizationStorage          storage.ResourceServer[*resourcemanagerpb.Organization]
	ProjectStorage               storage.ResourceServer[*resourcemanagerpb.Project]
	SchemaRegistry               *schema.Registry
	SubjectResolver              subject.Resolver
	RoleResolver                 role.Resolver
	AccessChecker                func(context.Context, *iampb.CheckAccessRequest) (*iampb.CheckAccessResponse, error)
	AuthenticationProvider       authentication.Provider
	DatabaseRoleResolver         role.DatabaseResolver
	SubjectExtractor             subject.Extractor
	ParentResolver               storage.ParentResolver
}

type ServerOptions struct {
	OpenFGAClient          openfgav1.OpenFGAServiceClient
	OpenFGAStoreID         string
	GRPCServer             grpc.ServiceRegistrar
	ServiceStorage         storage.ResourceServer[*iampb.Service]
	RoleStorage            storage.ResourceServer[*iampb.Role]
	PolicyStorage          storage.ResourceServer[*iampb.Policy]
	UserStorage            storage.ResourceServer[*iampb.User]
	OrganizationStorage    storage.ResourceServer[*resourcemanagerpb.Organization]
	ProjectStorage         storage.ResourceServer[*resourcemanagerpb.Project]
	SubjectResolver        subject.Resolver
	RoleResolver           role.Resolver
	SubjectExtractor       subject.Extractor
	AuthenticationProvider authentication.Provider
	DatabaseRoleResolver   role.DatabaseResolver
	ParentResolver         storage.ParentResolver
}

// Configures a new IAM Server
func NewServer(opts ServerOptions) error {
	schemaRegistry := &schema.Registry{
		Services: opts.ServiceStorage,
	}

	server := &Server{
		AuthorizationModelReconciler: &openfga.AuthorizationModelReconciler{
			StoreID:        opts.OpenFGAStoreID,
			Client:         opts.OpenFGAClient,
			ServiceStorage: opts.ServiceStorage,
		},
		PolicyReconciler: &openfga.PolicyReconciler{
			StoreID:         opts.OpenFGAStoreID,
			Client:          opts.OpenFGAClient,
			SchemaRegistry:  schemaRegistry,
			SubjectResolver: opts.SubjectResolver,
		},
		RoleReconciler: &openfga.RoleReconciler{
			StoreID:     opts.OpenFGAStoreID,
			Client:      opts.OpenFGAClient,
			RoleStorage: opts.RoleStorage,
		},
		SchemaRegistry:         schemaRegistry,
		OpenFGAClient:          opts.OpenFGAClient,
		OpenFGAStoreID:         opts.OpenFGAStoreID,
		ServiceStorage:         opts.ServiceStorage,
		RoleStorage:            opts.RoleStorage,
		PolicyStorage:          opts.PolicyStorage,
		UserStorage:            opts.UserStorage,
		OrganizationStorage:    opts.OrganizationStorage,
		ProjectStorage:         opts.ProjectStorage,
		SubjectResolver:        opts.SubjectResolver,
		RoleResolver:           opts.RoleResolver,
		AccessChecker:          openfga.AccessChecker(schemaRegistry, opts.OpenFGAClient, opts.OpenFGAStoreID),
		SubjectExtractor:       opts.SubjectExtractor,
		AuthenticationProvider: opts.AuthenticationProvider,
		DatabaseRoleResolver:   opts.DatabaseRoleResolver,
		ParentResolver:         opts.ParentResolver,
	}

	// Register all gRPC services with the gRPC server here.
	iamv1alphagrpc.RegisterIAMPolicyServer(opts.GRPCServer, server)
	iamv1alphagrpc.RegisterRolesServer(opts.GRPCServer, server)
	iamv1alphagrpc.RegisterServicesServer(opts.GRPCServer, server)
	iamv1alphagrpc.RegisterAccessCheckServer(opts.GRPCServer, server)
	iamv1alphagrpc.RegisterUsersServer(opts.GRPCServer, server)
	resourcemanagerv1alphagrpc.RegisterOrganizationsServer(opts.GRPCServer, server)
	resourcemanagerv1alphagrpc.RegisterProjectsServer(opts.GRPCServer, server)

	return nil
}

func RegisterProxyRoutes(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) {
	iamv1alphagateway.RegisterIAMPolicyHandler(ctx, mux, conn)
	iamv1alphagateway.RegisterRolesHandler(ctx, mux, conn)
	iamv1alphagateway.RegisterServicesHandler(ctx, mux, conn)
	iamv1alphagateway.RegisterAccessCheckHandler(ctx, mux, conn)
	iamv1alphagateway.RegisterUsersHandler(ctx, mux, conn)
	resourcemanagerv1alphagateway.RegisterOrganizationsHandler(ctx, mux, conn)
	resourcemanagerv1alphagateway.RegisterProjectsHandler(ctx, mux, conn)
}
