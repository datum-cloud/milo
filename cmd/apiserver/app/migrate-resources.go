package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
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

const (
	organizationPageSize = 99999
	projectPageSize      = 1000
	userPageSize         = 9999
)

func migrateResourcesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate-resources",
		Short: "Migrates organizations, projects, and users from the Datum OS API to the IAM service",
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

			projectStorage, err := postgres.ResourceServer(db, &resourcemanagerpb.Project{})
			if err != nil {
				return fmt.Errorf("failed to initialize project storage: %w", err)
			}

			policyStorage, err := postgres.ResourceServer(db, &iampb.Policy{})
			if err != nil {
				return err
			}

			userStorage, err := postgres.ResourceServer(db, &iampb.User{})
			if err != nil {
				return fmt.Errorf("failed to initialize user storage: %w", err)
			}

			datumOsBaseApiEndpoint, err := cmd.Flags().GetString("datum-os-api-endpoint")
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

			if command == "organizations" {
				slog.Info("Fetching organizations from Datum OS API", "baseEndpoint", datumOsBaseApiEndpoint, "pageSize", organizationPageSize)
				datumOsOrganizations := fetch.GetDatumOsOrganizations(
					datumOsBaseApiEndpoint,
					datumOsApiKey,
					organizationPageSize,
				)
				if datumOsOrganizations == nil {
					slog.Error("Failed to fetch organizations, or no organizations found. Aborting organization migration.")
				} else {
					slog.Info("Fetched organizations from Datum OS API", "count", len(datumOsOrganizations))
				}

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

					orgNameForNewSystem := datumOsOrg.Name
					if !strings.HasPrefix(orgNameForNewSystem, "organizations/") {
						if datumOsOrg.OrganizationID != "" {
							orgNameForNewSystem = fmt.Sprintf("organizations/%s", datumOsOrg.OrganizationID)
							slog.Warn("datumOsOrg.Name was not in expected format. Using OrganizationID to construct FQN.", "originalName", datumOsOrg.Name, "constructedName", orgNameForNewSystem)
						} else {
							slog.Error("datumOsOrg.Name is not in expected format and OrganizationID is empty. Skipping organization.", "datumOsOrgName", datumOsOrg.Name)
							continue
						}
					}

					owners := make([]string, len(datumOsOrg.Spec.Owners))
					for i, owner := range datumOsOrg.Spec.Owners {
						owners[i] = fmt.Sprintf("user:%s", owner.Email)
					}

					policy := &iampb.SetIamPolicyRequest{
						Policy: &iampb.Policy{
							Name: fmt.Sprintf("iam.datumapis.com/%s", orgNameForNewSystem),
							Spec: &iampb.PolicySpec{
								Bindings: []*iampb.Binding{{
									Role:    "services/iam.datumapis.com/roles/organizationManager",
									Members: owners,
								}},
							},
						},
					}

					_, err = setIamPolicy(cmd.Context(), policy, policyStorage, policyReconciler)
					if err != nil {
						slog.Error("Failed to set IAM policy for organization", "error", err, "organizationName", orgNameForNewSystem)
						continue
					}

					var migratedOrg *resourcemanagerpb.Organization
					slog.Info("Checking if organization already exists in new system", "organizationName", orgNameForNewSystem)
					existingOrg, getErr := organizationsStorage.GetResource(cmd.Context(), &storage.GetResourceRequest{Name: orgNameForNewSystem})

					if getErr == nil {
						slog.Info("Organization already exists in new system, using existing resource.", "organizationName", existingOrg.Name)
						migratedOrg = existingOrg
					} else if status.Code(getErr) == codes.NotFound {
						slog.Info("Organization not found in new system, proceeding with creation.", "organizationName", orgNameForNewSystem)
						createTime, parseCreateErr := parseTime(datumOsOrg.CreateTime)
						if parseCreateErr != nil {
							slog.Error("Failed to parse create time for organization", "error", parseCreateErr, "organizationName", orgNameForNewSystem)
							continue
						}
						updateTime, parseUpdateErr := parseTime(datumOsOrg.UpdateTime)
						if parseUpdateErr != nil {
							slog.Error("Failed to parse update time for organization", "error", parseUpdateErr, "organizationName", orgNameForNewSystem)
							continue
						}

						orgSimpleID := datumOsOrg.OrganizationID
						if orgSimpleID == "" {
							nameParts := strings.Split(orgNameForNewSystem, "/")
							if len(nameParts) == 2 {
								orgSimpleID = nameParts[1]
							} else {
								slog.Error("Cannot determine simple organization ID for OrganizationId field", "orgNameForNewSystem", orgNameForNewSystem)
								continue
							}
						}

						resourceToCreate := &resourcemanagerpb.Organization{
							Name:           orgNameForNewSystem,
							OrganizationId: orgSimpleID,
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
						}
						createdOrg, createErr := organizationsStorage.CreateResource(cmd.Context(), &storage.CreateResourceRequest[*resourcemanagerpb.Organization]{
							Resource: resourceToCreate,
							Name:     resourceToCreate.Name,
						})
						if createErr != nil {
							slog.Error("Failed to create organization after NotFound check", "error", createErr, "organizationName", orgNameForNewSystem)
							continue
						}
						slog.Info("Successfully created new organization.", "organizationName", createdOrg.Name)
						migratedOrg = createdOrg
					} else {
						slog.Error("Failed to check for existing organization", "error", getErr, "organizationName", orgNameForNewSystem)
						continue
					}

					if migratedOrg == nil {
						slog.Error("Organization could not be obtained. Skipping project migration for this org.", "organizationName", orgNameForNewSystem)
						continue
					}
					slog.Info("Organization obtained for project migration", "organizationName", migratedOrg.Name)

					slog.Info("Fetching projects for organization", "organizationOldID", datumOsOrg.OrganizationID, "baseEndpoint", datumOsBaseApiEndpoint, "pageSize", projectPageSize)
					oldAPIProjects := fetch.GetDatumOsProjects(
						datumOsBaseApiEndpoint,
						datumOsApiKey,
						datumOsOrg.OrganizationID,
						projectPageSize,
					)
					if oldAPIProjects == nil {
						slog.Warn("Failed to fetch projects for organization, or no projects found.", "organizationID", datumOsOrg.OrganizationID)
					} else {
						slog.Info("Fetched projects from old API", "count", len(oldAPIProjects), "organizationID", datumOsOrg.OrganizationID)
					}

					for k, oldProject := range oldAPIProjects {
						slog.Info("Attempting to migrate project", "index", k+1, "oldProjectResourceName", oldProject.GetName(), "oldProjectDisplayName", oldProject.GetDisplayName(), "oldProjectUID", oldProject.GetUid())

						var projectSimpleID string
						if oldProject.GetName() != "" {
							nameParts := strings.Split(oldProject.GetName(), "/")
							projectSimpleID = nameParts[len(nameParts)-1]
						} else if oldProject.GetUid() != "" {
							projectSimpleID = oldProject.GetUid()
							slog.Warn("Project from old system is missing a resource name (e.g. projects/id), using UID as Project Simple ID", "UID", oldProject.GetUid(), "DisplayName", oldProject.GetDisplayName())
						} else {
							slog.Error("Project from old system is missing both resource name and UID. Cannot determine Project Simple ID. Skipping.", "projectDetails", oldProject)
							continue
						}
						if projectSimpleID == "" {
							slog.Error("Derived projectSimpleID is empty. Skipping.", "oldProjectName", oldProject.GetName(), "oldProjectUID", oldProject.GetUid())
							continue
						}

						projectFQNForStorage := fmt.Sprintf("%s/projects/%s", migratedOrg.Name, projectSimpleID)
						projectResourceName := fmt.Sprintf("projects/%s", projectSimpleID)
						projectParentName := migratedOrg.Name

						slog.Info("Checking if project already exists in new system", "projectFQNForStorage", projectFQNForStorage)
						_, getProjectErr := projectStorage.GetResource(cmd.Context(), &storage.GetResourceRequest{Name: projectFQNForStorage})

						if getProjectErr == nil {
							slog.Warn("Project already exists in new system, skipping creation.", "projectFQNForStorage", projectFQNForStorage)
							continue
						} else if status.Code(getProjectErr) == codes.NotFound {
							slog.Info("Project not found in new system, proceeding with creation.", "projectFQNForStorage", projectFQNForStorage)
							migratedProjectResource := proto.Clone(oldProject).(*resourcemanagerpb.Project)

							migratedProjectResource.Name = projectResourceName
							migratedProjectResource.Parent = projectParentName
							migratedProjectResource.ProjectId = projectSimpleID

							createdProject, createProjectErr := projectStorage.CreateResource(cmd.Context(), &storage.CreateResourceRequest[*resourcemanagerpb.Project]{
								Name:     projectFQNForStorage,
								Parent:   projectParentName,
								Resource: migratedProjectResource,
							})
							if createProjectErr != nil {
								slog.Error("Failed to create project in new system after NotFound check", "error", createProjectErr, "projectFQNForStorage", projectFQNForStorage)
								continue
							}
							slog.Info("Project migrated into IAM System", "createdProjectName", createdProject.GetName(), "projectFQNForStorage", projectFQNForStorage)
						} else {
							slog.Error("Failed to check for existing project", "error", getProjectErr, "projectFQNForStorage", projectFQNForStorage)
							continue
						}
					}
				}
			}

			if command == "users" {
				slog.Info("Fetching users from Datum OS API", "baseEndpoint", datumOsBaseApiEndpoint, "pageSize", userPageSize)
				datumOsUsers := fetch.GetDatumOsUsers(
					datumOsBaseApiEndpoint,
					datumOsApiKey,
					userPageSize,
				)
				if datumOsUsers == nil {
					slog.Error("Failed to fetch users, or no users found. Aborting user migration.")
				} else {
					slog.Info("Fetched users from Datum OS API", "count", len(datumOsUsers))
				}

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

					sub, err := subjectResolver(cmd.Context(), subject.UserKind, datumOsUser.Spec.Email)
					if err != nil {
						if !errors.Is(err, subject.ErrSubjectNotFound) {
							slog.Error("Failed to resolve subject", "error", err, "user", datumOsUser)
							continue
						}
					}
					if len(sub) > 0 {
						slog.Error("User already exists", "user", datumOsUser.Spec.Email)
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
	cmd.Flags().String("datum-os-api-endpoint", "", "Base Endpoint for the Datum OS API (e.g., https://api.example.com/datum-os)")
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
