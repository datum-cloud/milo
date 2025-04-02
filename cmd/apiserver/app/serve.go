package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"

	"buf.build/gen/go/datum-cloud/iam/grpc/go/datum/iam/v1alpha/iamv1alphagrpc"
	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	_ "github.com/lib/pq"
	sqldblogger "github.com/simukti/sqldb-logger"
	"go.datum.net/iam/internal/grpc/auth"
	"go.datum.net/iam/internal/grpc/auth/jwt"
	"go.datum.net/iam/internal/grpc/errors"
	"go.datum.net/iam/internal/grpc/logging"
	"go.datum.net/iam/internal/grpc/recovery"
	iamServer "go.datum.net/iam/internal/grpc/server"
	"go.datum.net/iam/internal/role"
	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/storage/postgres"
	"go.datum.net/iam/internal/subject"
	"go.datum.net/iam/internal/tracing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"google.golang.org/genproto/googleapis/api/serviceconfig"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

func mustStringFlag(flags *pflag.FlagSet, flagName string) string {
	val, err := flags.GetString(flagName)
	if err != nil {
		panic(err)
	}
	return val
}

func serve() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serves the IAM gRPC service and REST Proxy",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
				Level:     slog.LevelDebug,
				AddSource: false,
			}))
			slog.SetDefault(logger)

			if err := tracing.Configure(cmd.Context(), resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String("iam.datumapis.com"),
			)); err != nil {
				return fmt.Errorf("failed to initialize tracing: %w", err)
			}

			authConfig, err := getAuthConfig(cmd)
			if err != nil {
				return fmt.Errorf("failed to get authentication config: %w", err)
			}

			subjectExtractor, err := jwt.SubjectExtractor(authConfig)
			if err != nil {
				return err
			}

			dsn := mustStringFlag(cmd.Flags(), "database")

			db, err := sql.Open("postgres", dsn)
			if err != nil {
				return err
			}

			db = sqldblogger.OpenDriver(dsn, db.Driver(), loggerFunc(func(ctx context.Context, level sqldblogger.Level, msg string, data map[string]interface{}) {
				slog.DebugContext(ctx, msg, slog.Any("data", data))
			}))

			openfgaClient, openfgaStore, err := getOpenFGAClient(cmd, slog.Default())
			if err != nil {
				return err
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			subjectResolver := subject.NoopResolver()

			serviceStorage, err := postgres.ResourceServer(db, &iampb.Service{})
			if err != nil {
				return err
			}

			roleStorage, err := postgres.ResourceServer(db, &iampb.Role{})
			if err != nil {
				return err
			}

			policyStorage, err := postgres.ResourceServer(db, &iampb.Policy{})
			if err != nil {
				return err
			}

			userStorage, err := postgres.ResourceServer(db, &iampb.User{})
			if err != nil {
				return err
			}

			roleResolver := role.Resolver(func(ctx context.Context, roleName string) error {
				_, err := roleStorage.GetResource(ctx, &storage.GetResourceRequest{
					Name: roleName,
				})
				return err
			})

			grpcListener, err := net.Listen("tcp", mustStringFlag(cmd.Flags(), "grpc-addr"))
			if err != nil {
				return err
			}

			slog.InfoContext(ctx, "creating a client connection to the IAM gRPC service that was started", slog.String("address", grpcListener.Addr().String()))
			grpcClientConn, err := grpc.NewClient(
				grpcListener.Addr().String(),
				grpc.WithSharedWriteBuffer(true),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
			)
			if err != nil {
				return err
			}

			parentResolverRegistry := &storage.ParentResolverRegistry{}
			// Register a new parent resolver for the Project resource.
			parentResolverRegistry.RegisterResolver(&iampb.Service{}, storage.ResourceParentResolver(serviceStorage))
			parentResolverRegistry.RegisterResolver(&iampb.User{}, storage.ResourceParentResolver(userStorage))

			unaryInterceptors := []grpc.UnaryServerInterceptor{
				errors.InternalErrorsInterceptor(slog.Default()),
				recovery.UnaryServerInterceptor(),
				logging.UnaryServerInterceptor(slog.Default()),
			}
			if disable, _ := cmd.Flags().GetBool("disable-auth"); !disable {
				unaryInterceptors = append(unaryInterceptors, auth.SubjectAuthorizationInterceptor(
					iamv1alphagrpc.NewAccessCheckClient(grpcClientConn),
					subjectExtractor,
					auth.ResourceNameResolver(),
					parentResolverRegistry,
				))
				roleResolver = role.IAMUseRoleResolver(iamv1alphagrpc.NewAccessCheckClient(grpcClientConn), subjectExtractor)
			}

			// Creates a new gRPC service with logging, error handling, and
			// authentication middlewares.
			grpcServer := grpc.NewServer(
				grpc.ChainUnaryInterceptor(unaryInterceptors...),
				grpc.StatsHandler(otelgrpc.NewServerHandler()),
			)

			// Creates a new IAM gRPC service and registers it with the gRPC server
			if err := iamServer.NewServer(iamServer.ServerOptions{
				OpenFGAClient:   openfgaClient,
				OpenFGAStoreID:  openfgaStore,
				GRPCServer:      grpcServer,
				ServiceStorage:  serviceStorage,
				RoleStorage:     roleStorage,
				PolicyStorage:   policyStorage,
				UserStorage:     userStorage,
				SubjectResolver: subjectResolver,
				RoleResolver:    roleResolver,
			}); err != nil {
				return fmt.Errorf("failed to create IAM gRPC server: %w", err)
			}

			go func() {
				slog.InfoContext(ctx, "starting gRPC server", slog.String("address", grpcListener.Addr().Network()))
				err := grpcServer.Serve(grpcListener)
				if err != nil {
					panic(err)
				}
			}()

			gRPCRestProxy := runtime.NewServeMux(
				runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
					MarshalOptions: protojson.MarshalOptions{
						EmitDefaultValues: true,
					},
				}),
			)

			// Register all REST proxy handlers here.
			iamServer.RegisterProxyRoutes(ctx, gRPCRestProxy, grpcClientConn)

			proxySrv := &http.Server{
				Addr:    mustStringFlag(cmd.Flags(), "rest-addr"),
				Handler: gRPCRestProxy,
			}

			slog.InfoContext(ctx, "starting REST proxy server", slog.String("address", proxySrv.Addr))
			return proxySrv.ListenAndServe()
		},
	}

	registerOpenFGAFlags(cmd.Flags())

	cmd.Flags().String("database", "", "Connection string to use when connecting to the database")
	cmd.Flags().String("grpc-addr", ":8080", "The listen address to use for the gRPC service")
	cmd.Flags().String("rest-addr", ":8081", "The listen address to use for the REST service")
	cmd.Flags().String("metrics-addr", ":9000", "The listen address to use for the metrics service")

	cmd.Flags().String("authentication-config", "", "Configuration file to use for authenticating API requests")
	cmd.Flags().Bool("disable-auth", false, "Whether authorization checks should be disabled on the APIs")

	return cmd
}

// Temp project getter func to make it so any project is valid.
type ResourceGetterFunc[T storage.Resource] func(context.Context, *storage.GetResourceRequest) (T, error)

func (f ResourceGetterFunc[T]) GetResource(ctx context.Context, req *storage.GetResourceRequest) (T, error) {
	return f(ctx, req)
}

func getAuthConfig(cmd *cobra.Command) (*serviceconfig.Authentication, error) {
	authConfigString, err := cmd.Flags().GetString("authentication-config")
	if err != nil {
		return nil, err
	} else if authConfigString == "" {
		return &serviceconfig.Authentication{}, nil
	}

	authConfig := &serviceconfig.Authentication{}
	return authConfig, protojson.Unmarshal([]byte(authConfigString), authConfig)
}

type loggerFunc func(ctx context.Context, level sqldblogger.Level, msg string, data map[string]interface{})

func (l loggerFunc) Log(ctx context.Context, level sqldblogger.Level, msg string, data map[string]interface{}) {
	l(ctx, level, msg, data)
}
