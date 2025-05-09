package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"buf.build/gen/go/datum-cloud/iam/grpc/go/datum/iam/v1alpha/iamv1alphagrpc"
	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	resourcemanagerpb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/resourcemanager/v1alpha"
	_ "github.com/lib/pq"
	sqldblogger "github.com/simukti/sqldb-logger"
	"go.datum.net/iam/internal/grpc/auth"
	"go.datum.net/iam/internal/grpc/auth/jwt"
	"go.datum.net/iam/internal/grpc/errors"
	"go.datum.net/iam/internal/grpc/logging"
	"go.datum.net/iam/internal/grpc/recovery"
	iamServer "go.datum.net/iam/internal/grpc/server"
	authProvider "go.datum.net/iam/internal/providers/authentication/zitadel"
	"go.datum.net/iam/internal/role"
	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/storage/postgres"
	"go.datum.net/iam/internal/subject"
	"go.datum.net/iam/internal/tracing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/zitadel-go/v3/pkg/client"
	"github.com/zitadel/zitadel-go/v3/pkg/zitadel"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"google.golang.org/genproto/googleapis/api/serviceconfig"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

			// Common HTTPS certificate and key files, declared once.
			tlsCertFile := mustStringFlag(cmd.Flags(), "tls-cert-file")
			tlsKeyFile := mustStringFlag(cmd.Flags(), "tls-key-file")

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

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

			organizationStorage, err := postgres.ResourceServer(db, &resourcemanagerpb.Organization{})
			if err != nil {
				return err
			}

			projectStorage, err := postgres.ResourceServer(db, &resourcemanagerpb.Project{})
			if err != nil {
				return err
			}

			subjectResolver, err := subject.DatabaseResolver(db)
			if err != nil {
				return fmt.Errorf("failed to create database resolver: %w", err)
			}

			subjectExtractor, err := jwt.SubjectExtractor(authConfig, subjectResolver)
			if err != nil {
				return err
			}

			roleResolver := role.Resolver(func(ctx context.Context, roleName string) error {
				_, err := roleStorage.GetResource(ctx, &storage.GetResourceRequest{
					Name: roleName,
				})
				return err
			})

			zitadelClient, err := getZitadelClient(cmd, ctx)
			if err != nil {
				return err
			}
			authenticationProvider := &authProvider.Zitadel{
				Client: zitadelClient,
			}

			grpcListener, err := net.Listen("tcp", mustStringFlag(cmd.Flags(), "grpc-addr"))
			if err != nil {
				return err
			}

			// Configure dial options for the gRPC client connection to the local gRPC service
			var dialOptions []grpc.DialOption
			dialOptions = append(dialOptions, grpc.WithSharedWriteBuffer(true))
			dialOptions = append(dialOptions, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))

			if tlsCertFile != "" && tlsKeyFile != "" {
				// If server TLS is configured, the client connection to it must also use TLS.
				// We use the server's certificate file for the client's trusted CA.
				slog.InfoContext(ctx, "gRPC client for REST proxy will use TLS", slog.String("serverCert", tlsCertFile))
				clientCreds, err := credentials.NewClientTLSFromFile(tlsCertFile, "localhost")
				if err != nil {
					return fmt.Errorf("failed to create client TLS credentials for gRPC proxy client: %w", err)
				}
				dialOptions = append(dialOptions, grpc.WithTransportCredentials(clientCreds))
			} else {
				slog.InfoContext(ctx, "gRPC client for REST proxy will use insecure credentials")
				dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
			}

			slog.InfoContext(ctx, "creating a client connection to the IAM gRPC service that was started", slog.String("address", grpcListener.Addr().String()))
			grpcClientConn, err := grpc.NewClient(
				grpcListener.Addr().String(),
				dialOptions...,
			)
			if err != nil {
				return err
			}

			parentResolverRegistry := &storage.ParentResolverRegistry{}
			// Register a new parent resolver for the Project resource.
			parentResolverRegistry.RegisterResolver(&iampb.Service{}, storage.ResourceParentResolver(serviceStorage))
			parentResolverRegistry.RegisterResolver(&iampb.User{}, storage.ResourceParentResolver(userStorage))
			parentResolverRegistry.RegisterResolver(&iampb.Role{}, storage.ResourceParentResolver(roleStorage))
			parentResolverRegistry.RegisterResolver(&resourcemanagerpb.Project{}, storage.ResourceParentResolver(projectStorage))
			parentResolverRegistry.RegisterResolver(&resourcemanagerpb.Organization{}, storage.ResourceParentResolver(organizationStorage))

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

			// Configure gRPC server options
			grpcServerOptions := []grpc.ServerOption{
				grpc.ChainUnaryInterceptor(unaryInterceptors...),
				grpc.StatsHandler(otelgrpc.NewServerHandler()),
			}

			// Add TLS credentials if gRPC TLS cert and key are provided
			if tlsCertFile != "" && tlsKeyFile != "" {
				creds, err := credentials.NewServerTLSFromFile(tlsCertFile, tlsKeyFile)
				if err != nil {
					return fmt.Errorf("failed to load gRPC TLS credentials from common certs: %w", err)
				}
				grpcServerOptions = append(grpcServerOptions, grpc.Creds(creds))
				slog.InfoContext(ctx, "gRPC server will use TLS using common certs")
			} else {
				slog.InfoContext(ctx, "gRPC server will not use TLS (common TLS cert/key not provided)")
			}

			// Creates a new gRPC service with logging, error handling, and
			// authentication middlewares.
			grpcServer := grpc.NewServer(grpcServerOptions...)

			// Creates a new IAM gRPC service and registers it with the gRPC server
			if err := iamServer.NewServer(iamServer.ServerOptions{
				OpenFGAClient:          openfgaClient,
				OpenFGAStoreID:         openfgaStore,
				GRPCServer:             grpcServer,
				ServiceStorage:         serviceStorage,
				RoleStorage:            roleStorage,
				PolicyStorage:          policyStorage,
				UserStorage:            userStorage,
				OrganizationStorage:    organizationStorage,
				ProjectStorage:         projectStorage,
				SubjectResolver:        subjectResolver,
				RoleResolver:           roleResolver,
				AuthenticationProvider: authenticationProvider,
				SubjectExtractor:       subjectExtractor,
				ParentResolver:         parentResolverRegistry,
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

			go func() {
				// Start TLS server if cert and key are provided
				if tlsCertFile != "" && tlsKeyFile != "" {
					slog.InfoContext(ctx, "starting REST proxy server with TLS using common certs", slog.String("address", proxySrv.Addr))
					if err := proxySrv.ListenAndServeTLS(tlsCertFile, tlsKeyFile); err != nil && err != http.ErrServerClosed {
						slog.ErrorContext(ctx, "failed to start REST proxy server", slog.Any("error", err))
						panic(err)
					}
				}

				slog.InfoContext(ctx, "starting REST proxy server without TLS (common TLS cert/key not provided)", slog.String("address", proxySrv.Addr))
				if err := proxySrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					slog.ErrorContext(ctx, "failed to start REST proxy server", slog.Any("error", err))
					panic(err)
				}
			}()

			// Start Metrics server
			metricsAddr := mustStringFlag(cmd.Flags(), "metrics-addr")
			metricsMux := http.NewServeMux()
			metricsMux.Handle("/metrics", promhttp.Handler())
			metricsSrv := &http.Server{
				Addr:    metricsAddr,
				Handler: metricsMux,
			}

			go func() {
				if tlsCertFile != "" && tlsKeyFile != "" {
					slog.InfoContext(ctx, "starting Metrics server with TLS using common certs", slog.String("address", metricsSrv.Addr))
					if err := metricsSrv.ListenAndServeTLS(tlsCertFile, tlsKeyFile); err != nil && err != http.ErrServerClosed {
						slog.ErrorContext(ctx, "failed to start HTTPS Metrics server", slog.Any("error", err))
						panic(err)
					}
				} else {
					slog.InfoContext(ctx, "starting Metrics server without TLS", slog.String("address", metricsSrv.Addr))
					if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
						slog.ErrorContext(ctx, "failed to start HTTP Metrics server", slog.Any("error", err))
						panic(err)
					}
				}
			}()

			// Keep the main function alive until context is cancelled, or wait for servers
			<-ctx.Done()
			// Add graceful shutdown for proxySrv and metricsSrv if needed here
			return nil
		},
	}

	registerOpenFGAFlags(cmd.Flags())

	cmd.Flags().String("database", "", "Connection string to use when connecting to the database")
	cmd.Flags().String("grpc-addr", ":8080", "The listen address to use for the gRPC service")
	cmd.Flags().String("rest-addr", ":8081", "The listen address to use for the REST service")
	cmd.Flags().String("metrics-addr", ":9000", "The listen address to use for the metrics service")

	cmd.Flags().String("tls-cert-file", "", "Path to the common TLS certificate file for all HTTPS/TLS services")
	cmd.Flags().String("tls-key-file", "", "Path to the common TLS key file for all HTTPS/TLS services")

	cmd.Flags().String("authentication-config", "", "Configuration file to use for authenticating API requests")
	cmd.Flags().Bool("disable-auth", false, "Whether authorization checks should be disabled on the APIs")

	cmd.Flags().String("zitadel-endpoint", "http://localhost:8082", "The domain of the ZITADEL instance")
	cmd.Flags().String("zitadel-key-path", "", "Path to the ZITADEL service account private key JSON file")

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

func getZitadelConfig(cmd *cobra.Command) (*authProvider.Config, error) {
	endpoint, err := cmd.Flags().GetString("zitadel-endpoint")
	if err != nil {
		return nil, err
	}

	keyPath, err := cmd.Flags().GetString("zitadel-key-path")
	if err != nil {
		return nil, err
	}

	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid ZITADEL endpoint: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("invalid ZITADEL endpoint scheme: %s. The scheme must be either 'http' or 'https'", parsedURL.Scheme)
	}

	insecure := false
	if parsedURL.Scheme == "http" {
		insecure = true
	}

	if insecure {
		if parsedURL.Port() == "" {
			return nil, fmt.Errorf("invalid domain format: %s. When using the --zitadel-insecure=true flag, the domain must include both the scheme (http), hostname and port in the format 'http://domain:port' (e.g., 'http://localhost:8082')", endpoint)
		}
	}

	return &authProvider.Config{
		Endpoint: parsedURL.Hostname(),
		Port:     parsedURL.Port(),
		KeyPath:  keyPath,
		Insecure: insecure,
	}, nil
}

func getZitadelClient(cmd *cobra.Command, ctx context.Context) (*client.Client, error) {
	zitadelConfig, err := getZitadelConfig(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get ZITADEL config: %w", err)
	}

	if zitadelConfig.Insecure {
		slog.InfoContext(ctx, "creating an insecure client connection to Zitadel authentication provider", slog.String("endpoint", zitadelConfig.Endpoint))
	} else {
		slog.InfoContext(ctx, "creating an secure client connection to Zitadel authentication provider", slog.String("endpoint", zitadelConfig.Endpoint))
	}

	zitadelOptions := []zitadel.Option{}
	if zitadelConfig.Insecure {
		zitadelOptions = append(zitadelOptions, zitadel.WithInsecure(zitadelConfig.Port))
	}

	if zitadelConfig.Port != "" {
		port, err := strconv.Atoi(zitadelConfig.Port)
		if err != nil {
			return nil, fmt.Errorf("failed to convert ZITADEL port to int: %w", err)
		}
		zitadelOptions = append(zitadelOptions, zitadel.WithPort(uint16(port)))
	}

	zitadelClient, err := client.New(ctx, zitadel.New(zitadelConfig.Endpoint, zitadelOptions...),
		client.WithAuth(client.DefaultServiceUserAuthentication(zitadelConfig.KeyPath, oidc.ScopeOpenID, client.ScopeZitadelAPI())),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create zitadel client: %w", err)
	}

	return zitadelClient, nil
}
