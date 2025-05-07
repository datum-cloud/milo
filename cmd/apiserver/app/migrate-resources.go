package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	resourcemanagerpb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/resourcemanager/v1alpha"
	"go.datum.net/iam/datum-os-migration/fetch"
	"go.datum.net/iam/internal/providers/openfga"
	"go.datum.net/iam/internal/schema"
	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/storage/postgres"
	"go.datum.net/iam/internal/subject"
)

func migrateResourcesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate-resources",
		Short: "Migrates organizations and users from the Datum OS API to the IAM service",
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

			subjectResolver, err := subject.DatabaseResolver(db)
			if err != nil {
				return fmt.Errorf("failed to create database resolver: %w", err)
			}

			policyReconciler := &openfga.PolicyReconciler{
				StoreID: storeID,
				Client:  openFGAClient,
				SchemaRegistry: &schema.Registry{
					Services: serviceStorage,
				},
				SubjectResolver: subjectResolver,
			}

			organizationsStorage, err := postgres.ResourceServer(db, &resourcemanagerpb.Organization{})
			if err != nil {
				return fmt.Errorf("failed to initialize organization storage: %w", err)
			}

			policyStorage, err := postgres.ResourceServer(db, &iampb.Policy{})
			if err != nil {
				return err
			}

			userStorage, err := postgres.ResourceServer(db, &iampb.User{})
			if err != nil {
				return fmt.Errorf("failed to initialize user storage: %w", err)
			}

			datumOsApiEndpoint, err := cmd.Flags().GetString("datum-os-api-endpoint")
			if err != nil {
				return fmt.Errorf("failed to get `--datum-os-api-endpoint`: %w", err)
			}

			datumOsApiKey, err := cmd.Flags().GetString("datum-os-api-key")
			if err != nil {
				return fmt.Errorf("failed to get `--datum-os-api-key`: %w", err)
			}

			command, err := cmd.Flags().GetString("migrate-command")
			if err != nil {
				return fmt.Errorf("failed to get `--migrate-command`: %w", err)
			}
			if command != "organizations" && command != "users" {
				return fmt.Errorf("invalid command: %s. Must be one of: organizations, users", command)
			}

			// Migrate organizations
			if command == "organizations" {
				datumOsOrganizations := fetch.GetDatumOsOrganizations(
					datumOsApiEndpoint,
					datumOsApiKey,
				)

				for i, datumOsOrg := range datumOsOrganizations {
					fmt.Printf("--- Organization %d ---\n", i+1)
					fmt.Printf("  Name: %s\n", datumOsOrg.Name)
					fmt.Printf("  DisplayName: %s\n", datumOsOrg.DisplayName)
					fmt.Printf("  OrganizationID: %s\n", datumOsOrg.OrganizationID)
					fmt.Printf("  UID: %s\n", datumOsOrg.UID)
					fmt.Printf("  Etag: %s\n", datumOsOrg.Etag)
					fmt.Printf("  Reconciling: %t\n", datumOsOrg.Reconciling)
					fmt.Printf("  CreateTime: %s\n", datumOsOrg.CreateTime)
					fmt.Printf("  UpdateTime: %s\n", datumOsOrg.UpdateTime)
					fmt.Printf("  Spec:\n")
					fmt.Printf("    AvatarRemoteURI: %s\n", datumOsOrg.Spec.AvatarRemoteURI)
					fmt.Printf("    Description: %s\n", datumOsOrg.Spec.Description)
					fmt.Printf("    ParentOrganizationID: %s\n", datumOsOrg.Spec.ParentOrganizationID)
					fmt.Printf("    Members (%d):\n", len(datumOsOrg.Spec.Members))
					for j, member := range datumOsOrg.Spec.Members {
						fmt.Printf("      Member %d: Name: %s %s, Email: %s, Role: %s, UID: %s, UserID: %s\n", j+1, member.Firstname, member.Lastname, member.Email, member.Role, member.UID, member.UserID)
					}
					fmt.Printf("    Owners (%d):\n", len(datumOsOrg.Spec.Owners))
					for k, owner := range datumOsOrg.Spec.Owners {
						fmt.Printf("      Owner %d: Name: %s %s, Email: %s, Role: %s, UID: %s, UserID: %s\n", k+1, owner.Firstname, owner.Lastname, owner.Email, owner.Role, owner.UID, owner.UserID)
					}
					fmt.Printf("  Status:\n")
					fmt.Printf("    Internal: %t\n", datumOsOrg.Status.Internal)
					fmt.Printf("    Personal: %t\n", datumOsOrg.Status.Personal)
					fmt.Printf("    VerificationState: %s\n", datumOsOrg.Status.VerificationState)
					fmt.Println("")

					owners := make([]string, len(datumOsOrg.Spec.Owners))
					for i, owner := range datumOsOrg.Spec.Owners {
						owners[i] = fmt.Sprintf("user:%s", owner.Email)
					}

					policy := &iampb.SetIamPolicyRequest{
						Policy: &iampb.Policy{
							Name: fmt.Sprintf("iam.datumapis.com/%s", datumOsOrg.Name),
							Spec: &iampb.PolicySpec{
								Bindings: []*iampb.Binding{{
									Role:    "services/iam.datumapis.com/roles/organizationManager",
									Members: owners,
								}},
							},
						},
					}

					// We set the IAM policy before creating the organization to ensure that the
					// organization oweners are already in the IAM system.
					_, err = setIamPolicy(cmd.Context(), policy, policyStorage, policyReconciler)
					if err != nil {
						slog.Error("Failed to set IAM policy", "error", err, "organization", datumOsOrg)
						continue
					}

					createTime, err := parseTime(datumOsOrg.CreateTime)
					if err != nil {
						slog.Error("Failed to parse create time", "error", err, "user", datumOsOrg)
						continue
					}

					updateTime, err := parseTime(datumOsOrg.UpdateTime)
					if err != nil {
						slog.Error("Failed to parse update time", "error", err, "user", datumOsOrg)
						continue
					}

					migratedOrg, err := organizationsStorage.CreateResource(cmd.Context(), &storage.CreateResourceRequest[*resourcemanagerpb.Organization]{
						Resource: &resourcemanagerpb.Organization{
							Name:           datumOsOrg.Name,
							OrganizationId: datumOsOrg.OrganizationID,
							Uid:            datumOsOrg.UID,
							DisplayName:    datumOsOrg.DisplayName,
							CreateTime:     timestamppb.New(createTime),
							UpdateTime:     timestamppb.New(updateTime),
							Reconciling:    datumOsOrg.Reconciling,
							Spec: &resourcemanagerpb.Organization_Spec{
								Description: datumOsOrg.Spec.Description,
							},
							Status: &resourcemanagerpb.Organization_Status{
								VerificationState: resourcemanagerpb.VerificationState_VERIFICATION_STATE_PENDING,
								Internal:          datumOsOrg.Status.Internal,
								Personal:          datumOsOrg.Status.Personal,
							},
						},
					})
					if err != nil {
						slog.Error("Failed to create organization", "error", err, "organization", datumOsOrg)
						continue
					}

					slog.Info("Organization migrated into IAM System", "organization", migratedOrg)
				}
			}

			// Migrate users
			if command == "users" {
				datumOsUsers := fetch.GetDatumOsUsers(
					datumOsApiEndpoint,
					datumOsApiKey,
				)

				for i, datumOsUser := range datumOsUsers {
					fmt.Printf("--- Migrating User %d ---\n", i+1)
					fmt.Printf("  Name: %s\n", datumOsUser.Name)
					fmt.Printf("  DisplayName: %s\n", datumOsUser.DisplayName)
					fmt.Printf("  UserID: %s\n", datumOsUser.UserID)
					fmt.Printf("  UID: %s\n", datumOsUser.UID)
					fmt.Printf("  Role: %s\n", datumOsUser.Spec.Role)
					fmt.Printf("  Status: %s\n", datumOsUser.Status.Status)
					fmt.Printf("  CreateTime: %s\n", datumOsUser.CreateTime)
					fmt.Printf("  UpdateTime: %s\n", datumOsUser.UpdateTime)
					fmt.Printf("  FirstName: %s\n", datumOsUser.Spec.Firstname)
					fmt.Printf("  LastName: %s\n", datumOsUser.Spec.Lastname)
					fmt.Printf("  PhoneNumbers: %v\n", datumOsUser.Spec.PhoneNumbers)
					fmt.Printf("  Orgs: %+v\n", datumOsUser.Spec.Orgs)
					fmt.Printf("  AvatarRemoteURI: %s\n", datumOsUser.Spec.AvatarRemoteURI)
					fmt.Printf("  AvatarUpdateTime: %s\n", datumOsUser.Spec.AvatarUpdateTime)
					fmt.Printf("  LastSeenTime: %s\n", datumOsUser.Spec.LastSeenTime)
					fmt.Printf("  Setting: %+v\n", datumOsUser.Spec.Setting)

					createTime, err := parseTime(datumOsUser.CreateTime)
					if err != nil {
						slog.Error("Failed to parse create time", "error", err, "user", datumOsUser)
						continue
					}

					updateTime, err := parseTime(datumOsUser.UpdateTime)
					if err != nil {
						slog.Error("Failed to parse update time", "error", err, "user", datumOsUser)
						continue
					}

					migratedUser, err := userStorage.CreateResource(cmd.Context(), &storage.CreateResourceRequest[*iampb.User]{
						Resource: &iampb.User{
							Name:        datumOsUser.Name,
							UserId:      datumOsUser.UserID,
							Uid:         datumOsUser.UID,
							DisplayName: datumOsUser.DisplayName,
							Annotations: map[string]string{
								"internal.iam.datumapis.com/zitadel-id": "pending",
							},
							Spec: &iampb.UserSpec{
								Email:      datumOsUser.Spec.Email,
								GivenName:  datumOsUser.Spec.Firstname,
								FamilyName: datumOsUser.Spec.Lastname,
							},
							CreateTime:  timestamppb.New(createTime),
							UpdateTime:  timestamppb.New(updateTime),
							Reconciling: datumOsUser.Reconciling,
						},
					})
					if err != nil {
						slog.Error("Failed to create user", "error", err, "user", datumOsUser)
						continue
					}

					policy := &iampb.SetIamPolicyRequest{
						Policy: &iampb.Policy{
							Name: fmt.Sprintf("iam.datumapis.com/%s", migratedUser.Name),
							Spec: &iampb.PolicySpec{
								Bindings: []*iampb.Binding{{
									Role:    "services/iam.datumapis.com/roles/userSelfManage",
									Members: []string{fmt.Sprintf("user:%s", migratedUser.Spec.Email)},
								}},
							},
						},
					}

					_, err = setIamPolicy(cmd.Context(), policy, policyStorage, policyReconciler)
					if err != nil {
						slog.Error("Failed to set IAM policy", "error", err, "user", migratedUser)
						continue
					}

					slog.Info("User migrated into IAM System", "user", migratedUser)
				}
			}

			return nil
		},
	}

	cmd.Flags().String("database-connection-string", "", "The connection string to use when connecting to the database used by the IAM service")
	cmd.Flags().String("openfga-endpoint", "", "The gRPC endpoint to use for communication to OpenFGA")
	cmd.Flags().String("openfga-ca", "", "The base64 encoded Certificate to use as the certificate authority when connecting to OpenFGA. Insecure mode will be used if this is not provided.")
	cmd.Flags().String("openfga-auth", "", "The auth token to use when communicating to OpenFGA")
	cmd.Flags().String("datum-os-api-endpoint", "", "Endpoint for the Datum OS API")
	cmd.Flags().String("datum-os-api-key", "", "API Key for the Datum OS API")
	cmd.Flags().String("openfga-store-id", "", "The ID of the store to use in the OpenFGA backend")
	cmd.Flags().String("migrate-command", "", "The command to run. Must be one of: organizations, users")

	return cmd
}

func parseTime(timeString string) (time.Time, error) {
	return time.Parse(time.RFC3339, timeString)
}

func setIamPolicy(ctx context.Context, req *iampb.SetIamPolicyRequest, policyStorage storage.ResourceServer[*iampb.Policy], policyReconciler *openfga.PolicyReconciler) (*iampb.Policy, error) {
	policy := req.Policy
	policy.UpdateTime = timestamppb.Now()

	policyExists := false
	_, err := policyStorage.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Policy.Name,
	})
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, err
	} else if err == nil {
		policyExists = true
	}

	if !policyExists {
		policy.Uid = uuid.NewString()
		policy, err = policyStorage.CreateResource(ctx, &storage.CreateResourceRequest[*iampb.Policy]{
			Name:     req.Policy.Name,
			Resource: policy,
		})
	} else {
		policy, err = policyStorage.UpdateResource(ctx, &storage.UpdateResourceRequest[*iampb.Policy]{
			Name: req.Policy.Name,
			Updater: func(existing *iampb.Policy) (new *iampb.Policy, err error) {
				return policy, nil
			},
		})
	}
	if err != nil {
		return nil, err
	}

	if err := policyReconciler.ReconcilePolicy(ctx, req.Policy.Name, policy); err != nil {
		return nil, err
	}

	return policy, nil
}
