package server

import (
	"context"

	"buf.build/gen/go/datum-cloud/iam/grpc-ecosystem/gateway/v2/datum/iam/v1alpha/iamv1alphagateway"
	"buf.build/gen/go/datum-cloud/iam/grpc/go/datum/iam/v1alpha/iamv1alphagrpc"
	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
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

	PolicyReconciler             *openfga.PolicyReconciler
	RoleReconciler               *openfga.RoleReconciler
	AuthorizationModelReconciler *openfga.AuthorizationModelReconciler
	OpenFGAClient                openfgav1.OpenFGAServiceClient
	OpenFGAStoreID               string
	ServiceStorage               storage.ResourceServer[*iampb.Service]
	RoleStorage                  storage.ResourceServer[*iampb.Role]
	PolicyStorage                storage.ResourceServer[*iampb.Policy]
	SchemaRegistry               *schema.Registry
	SubjectResolver              subject.Resolver
	RoleResolver                 role.Resolver
	AccessChecker                func(context.Context, *iampb.CheckAccessRequest) (*iampb.CheckAccessResponse, error)
}

type ServerOptions struct {
	OpenFGAClient   openfgav1.OpenFGAServiceClient
	OpenFGAStoreID  string
	GRPCServer      grpc.ServiceRegistrar
	ServiceStorage  storage.ResourceServer[*iampb.Service]
	RoleStorage     storage.ResourceServer[*iampb.Role]
	PolicyStorage   storage.ResourceServer[*iampb.Policy]
	SubjectResolver subject.Resolver
	RoleResolver    role.Resolver
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
			StoreID: opts.OpenFGAStoreID,
			Client:  opts.OpenFGAClient,
		},
		SchemaRegistry:  schemaRegistry,
		OpenFGAClient:   opts.OpenFGAClient,
		OpenFGAStoreID:  opts.OpenFGAStoreID,
		ServiceStorage:  opts.ServiceStorage,
		RoleStorage:     opts.RoleStorage,
		PolicyStorage:   opts.PolicyStorage,
		SubjectResolver: opts.SubjectResolver,
		RoleResolver:    opts.RoleResolver,
		AccessChecker:   openfga.AccessChecker(schemaRegistry, opts.OpenFGAClient, opts.OpenFGAStoreID),
	}

	// Register all gRPC services with the gRPC server here.
	iamv1alphagrpc.RegisterIAMPolicyServer(opts.GRPCServer, server)
	iamv1alphagrpc.RegisterRolesServer(opts.GRPCServer, server)
	iamv1alphagrpc.RegisterServicesServer(opts.GRPCServer, server)
	iamv1alphagrpc.RegisterAccessCheckServer(opts.GRPCServer, server)

	return nil
}

func RegisterProxyRoutes(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) {
	iamv1alphagateway.RegisterIAMPolicyHandler(ctx, mux, conn)
	iamv1alphagateway.RegisterRolesHandler(ctx, mux, conn)
	iamv1alphagateway.RegisterServicesHandler(ctx, mux, conn)
	iamv1alphagateway.RegisterAccessCheckHandler(ctx, mux, conn)
}
