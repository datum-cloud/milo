package app

import (
	"context"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"

	"buf.build/go/protoyaml"
	openfgav1grpc "github.com/openfga/api/proto/openfga/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/oauth2"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"go.datum.net/iam/internal/grpc/logging"
	"go.datum.net/iam/internal/grpc/validation"
	"go.datum.net/iam/internal/providers/openfga"
	"go.datum.net/iam/internal/schema"
	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/storage/postgres"
	"go.datum.net/iam/internal/subject"
)

func addResourcesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-resources",
		Short: "Load IAM resources into the system",
		Long: `
This command supports adding IAM resources (Services, Roles, and Policies) into the IAM system directly instead
of going through the API. This can be useful for loading default resources, or resources that require elevated access
to the system.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
				Level:     slog.LevelDebug,
				AddSource: false,
			}))
			slog.SetDefault(logger)

			dbConnectionString, err := cmd.Flags().GetString("database-connection-string")
			if err != nil {
				return fmt.Errorf("failed to get `--database-connection-string`: %w", err)
			}

			db, err := sql.Open("postgres", dbConnectionString)
			if err != nil {
				return fmt.Errorf("failed to open database connection: %w", err)
			}

			openFGAClient, storeID, err := getOpenFGAClient(cmd, logger)
			if err != nil {
				return err
			}

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

			authorizationModelReconciler := &openfga.AuthorizationModelReconciler{
				StoreID:        storeID,
				Client:         openFGAClient,
				ServiceStorage: serviceStorage,
			}

			roleReconciler := &openfga.RoleReconciler{
				StoreID:     storeID,
				Client:      openFGAClient,
				RoleStorage: roleStorage,
			}

			subjectResolver, err := subject.DatabaseResolver(db)
			if err != nil {
				return fmt.Errorf("failed to create database subject resolver: %w", err)
			}

			policyReconciler := &openfga.PolicyReconciler{
				StoreID: storeID,
				Client:  openFGAClient,
				SchemaRegistry: &schema.Registry{
					Services: serviceStorage,
				},
				SubjectResolver: subjectResolver,
			}

			// Add any IAM services that are defined and reconcile the authorization
			// model in OpenFGA.
			if err := addResources(cmd, serviceStorage,
				func(service *iampb.Service) error {
					if errs := validation.ValidateService(service); len(errs) > 0 {
						return errs.GRPCStatus().Err()
					}
					return nil
				}, func(*iampb.Service) error {
					// TODO: Move to temporal workflow.
					return authorizationModelReconciler.ReconcileAuthorizationModel(cmd.Context())
				}); err != nil {
				return fmt.Errorf("failed to add services: %w", err)
			}

			if err := addResources(cmd, roleStorage, func(role *iampb.Role) error {
				if errs := validation.ValidateRole(role, &validation.RoleValidatorOptions{
					PermissionValidator: validation.NewPermissionValidator(serviceStorage),
					RoleValidator:       validation.NewRoleValidator(roleStorage),
				}); len(errs) > 0 {
					return errs.GRPCStatus().Err()
				}
				return nil
			}, func(role *iampb.Role) error {
				return roleReconciler.ReconcileRole(cmd.Context(), role)
			}); err != nil {
				return fmt.Errorf("failed to add roles: %w", err)
			}

			if err := addResources(cmd, policyStorage, func(policy *iampb.Policy) error {
				if errs := validation.ValidatePolicy(policy, validation.PolicyValidatorOptions{
					RoleResolver: func(_ context.Context, role string) error {
						_, err := roleStorage.GetResource(context.Background(), &storage.GetResourceRequest{
							Name: role,
						})
						return err
					},
					SubjectResolver: subjectResolver,
					Context:         context.Background(),
				}); len(errs) > 0 {
					return errs.GRPCStatus().Err()
				}
				return nil
			}, func(policy *iampb.Policy) error {
				return policyReconciler.ReconcilePolicy(cmd.Context(), policy.Name, policy)
			}); err != nil {
				return fmt.Errorf("failed to add policies: %w", err)
			}

			if err := addResources(cmd, userStorage, func(user *iampb.User) error {
				if errs := validation.ValidateUser(user); len(errs) > 0 {
					return errs.GRPCStatus().Err()
				}

				subjectReference, err := subjectResolver(cmd.Context(), fmt.Sprintf("user:%s", user.Spec.Email))
				if err != nil {
					if !errors.Is(err, subject.ErrSubjectNotFound) {
						return fmt.Errorf("failed to create users: %w", err)
					}
				}
				if !errors.Is(err, subject.ErrSubjectNotFound) {
					if subjectReference.ResourceName != user.Name {
						return fmt.Errorf("user with email %s already exists under the resource name of: %s", user.Spec.Email, subjectReference.ResourceName)
					}
				}

				return nil
			}, func(user *iampb.User) error {
				return nil
			}); err != nil {
				return fmt.Errorf("failed to add users: %w", err)
			}

			return nil
		},
	}

	registerOpenFGAFlags(cmd.Flags())

	cmd.Flags().String("database-connection-string", "", "The connection string to use when connecting to the database used by the IAM service")
	cmd.Flags().Bool("overwrite", false, "Must be set to true to overwrite a resource when it already exists")
	cmd.Flags().String("services", "", "A directory that contains a series of YAML files with IAM service definitions that should be created.")
	cmd.Flags().String("roles", "", "A directory that contains a series of YAML files with IAM role definitions that should be created.")
	cmd.Flags().String("policies", "", "A directory that contains a series of YAML files with IAM policy definitions that should be created.")
	cmd.Flags().String("users", "", "A directory that contains a series of YAML files with IAM user definitions that should be created.")
	return cmd
}

// Loads a set of protobuf resources from YAML files in a directory.
//
// The protobuf message must have the `google.api.resource` protobuf annotation
// configured. The `plural` resource descriptor option will be used as the flag
// on the command to load the directory that should be used to load resource
// files.
//
// The `afterSave` function will be called once the resource has been
// successfully saved. Any errors returned will prevent any additional resources
// from being added.
func addResources[T storage.Resource](cmd *cobra.Command, resourceStorage storage.ResourceServer[T], validator func(T) error, afterSave func(T) error) error {
	var resourceType T
	protoReflector := resourceType.ProtoReflect()
	protoDescriptor := protoReflector.Descriptor()

	overwrite, err := cmd.Flags().GetBool("overwrite")
	if err != nil {
		return err
	}

	if !proto.HasExtension(protoDescriptor.Options(), annotations.E_Resource) {
		return fmt.Errorf("resource `%s` does not have the required `google.api.resource` annotation", protoDescriptor.FullName())
	}

	resourceDescriptor := proto.GetExtension(protoDescriptor.Options(), annotations.E_Resource).(*annotations.ResourceDescriptor)

	resourceDirectory, err := cmd.Flags().GetString(resourceDescriptor.Plural)
	if err != nil {
		return errors.New("flag `%` is not defined on the command")
	} else if resourceDirectory == "" {
		// Nothing to do if no directory was provided.
		return nil
	}

	resources, err := loadResources[T](resourceDirectory)
	if err != nil {
		return fmt.Errorf("failed to load resources: %w", err)
	}

	for _, resource := range resources {
		resourceName := resource.ProtoReflect().Get(protoDescriptor.Fields().ByName("name")).String()

		resourceLogger := slog.Default().With(
			slog.String("resource_name", resourceName),
			slog.String("resource_type", string(protoDescriptor.FullName())),
		)

		if err := validator(resource); err != nil {
			return fmt.Errorf("failed to validate resource: %w", err)
		}

		resourceExists := false
		if _, err := resourceStorage.GetResource(cmd.Context(), &storage.GetResourceRequest{
			Name: resourceName,
		}); err != nil && status.Code(err) != codes.NotFound {
			return err
		} else if err == nil {
			resourceExists = true
		}

		// Create the resource if it doesn't already exist, otherwise update when
		// the user indicated the resource should be overwritten.
		if !resourceExists {
			_, err = resourceStorage.CreateResource(cmd.Context(), &storage.CreateResourceRequest[T]{
				Name:     resourceName,
				Resource: resource,
			})
		} else {
			if overwrite {
				_, err = resourceStorage.UpdateResource(cmd.Context(), &storage.UpdateResourceRequest[T]{
					Name: resourceName,
					Updater: func(existing T) (new T, err error) {
						return resource, nil
					},
				})
			} else {
				resourceLogger.WarnContext(
					cmd.Context(),
					"Resource exists but `--overwrite` flag was not set to true. Skipping update.",
				)
				continue
			}
		}

		if err != nil {
			return fmt.Errorf("failed to create or update resource: %w", err)
		}

		resourceLogger.InfoContext(cmd.Context(), "successfully stored resource", slog.Any("resource", resource))

		if err := afterSave(resource); err != nil {
			return err
		}
	}

	return nil
}

func loadResources[T proto.Message](resourceDirectory string) ([]T, error) {
	files, err := os.ReadDir(resourceDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	resources := []T{}
	for _, file := range files {
		fullFilePath := path.Join(resourceDirectory, file.Name())

		fileInfo, err := os.Stat(fullFilePath)
		if err != nil {
			return nil, err
		} else if fileInfo.IsDir() {
			continue
		}

		contents, err := os.ReadFile(fullFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read resource file: %w", err)
		}

		unmarshaller := &protoyaml.UnmarshalOptions{
			DiscardUnknown: false,
			AllowPartial:   false,
			Path:           fullFilePath,
		}

		var resource T
		resource = resource.ProtoReflect().New().Interface().(T)
		if err := unmarshaller.Unmarshal(contents, resource); err != nil {
			return nil, fmt.Errorf("failed to unmarshal resource file: %w", err)
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

func getOpenFGAClient(cmd *cobra.Command, logger *slog.Logger) (openfgav1grpc.OpenFGAServiceClient, string, error) {
	endpoint, err := cmd.Flags().GetString("openfga-endpoint")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get OpenFGA endpoint config: %w", err)
	} else if endpoint == "" {
		return nil, "", fmt.Errorf("must provide `openfga-endpoint` config")
	}

	openfgaCa, err := cmd.Flags().GetString("openfga-ca")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get OpenFGA secure option: %w", err)
	}

	dialOptions := []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(
			logging.UnaryClientInterceptor(logger),
		),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}

	authToken, err := cmd.Flags().GetString("openfga-auth")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get OpenFGA auth config: %w", err)
	} else if authToken != "" {
		dialOptions = append(dialOptions, grpc.WithPerRPCCredentials(oauth.TokenSource{
			TokenSource: oauth2.StaticTokenSource(&oauth2.Token{
				AccessToken: authToken,
			}),
		}))
	}

	if openfgaCa != "" {
		decodedCert, err := base64.StdEncoding.DecodeString(openfgaCa)
		if err != nil {
			return nil, "", fmt.Errorf("failed to base64 decode cert: %w", err)
		}
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM([]byte(decodedCert)) {
			return nil, "", fmt.Errorf("failed to add grpc certificate authority option to the cert pool")
		}
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(certPool, "")))
	} else {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	openfgaClientConn, err := grpc.NewClient(endpoint, dialOptions...)
	if err != nil {
		return nil, "", err
	}

	storeID, err := cmd.Flags().GetString("openfga-store-id")
	if err != nil {
		return nil, "", fmt.Errorf("could not get OpenFGA store ID: %w", err)
	} else if storeID == "" {
		return nil, "", fmt.Errorf("store ID is required")
	}

	return openfgav1grpc.NewOpenFGAServiceClient(openfgaClientConn), storeID, nil
}

func registerOpenFGAFlags(flags *pflag.FlagSet) {
	flags.String("openfga-store-id", "", "The ID of the store to use in the OpenFGA backend")
	flags.String("openfga-endpoint", "", "The gRPC endpoint to use for communication to OpenFGA")
	flags.String("openfga-auth", "", "The auth token to use when communicating to OpenFGA")
	flags.String("openfga-ca", "", "The base64 encoded Certificate to use as the certificate authority when connecting to OpenFGA. Insecure mode will be used if this is not provided.")
}
