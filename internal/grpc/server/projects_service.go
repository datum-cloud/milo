package server

import (
	"context"
	"fmt"

	resourcemanagerpb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/resourcemanager/v1alpha"
	"github.com/mennanov/fmutils"
	"go.datum.net/iam/internal/grpc/longrunning"
	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/validation"
	lropb "google.golang.org/genproto/googleapis/longrunning"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateProject creates a new project resource.
func (s *Server) CreateProject(ctx context.Context, req *resourcemanagerpb.CreateProjectRequest) (*lropb.Operation, error) {
	if req.GetParent() == "" {
		return nil, status.Error(codes.InvalidArgument, "parent is required")
	}
	if req.GetProjectId() == "" {
		return nil, status.Error(codes.InvalidArgument, "project_id is required")
	}
	if req.GetProject() == nil {
		return nil, status.Error(codes.InvalidArgument, "project is required")
	}
	if req.GetProject().GetDisplayName() == "" {
		return nil, status.Error(codes.InvalidArgument, "project.display_name is required")
	}

	if errs := validation.ValidateProject(req.GetProject()); len(errs) > 0 {
		return nil, errs.GRPCStatus().Err()
	}

	if req.GetValidateOnly() {
		metadata := &resourcemanagerpb.CreateProjectMetadata{}
		return longrunning.ResponseOperation(metadata, req.GetProject(), true)
	}

	projectToCreate := proto.Clone(req.GetProject()).(*resourcemanagerpb.Project)
	projectToCreate.Name = fmt.Sprintf("projects/%s", req.GetProjectId())
	projectToCreate.Parent = req.GetParent()

	now := timestamppb.Now()
	projectToCreate.CreateTime = now
	projectToCreate.UpdateTime = now

	createdProject, err := s.ProjectStorage.CreateResource(ctx, &storage.CreateResourceRequest[*resourcemanagerpb.Project]{
		Parent:   req.GetParent(),
		Resource: projectToCreate,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create project: %v", err)
	}

	metadata := &resourcemanagerpb.CreateProjectMetadata{}
	return longrunning.ResponseOperation(metadata, createdProject, true)
}

// GetProject retrieves a project resource by its name.
func (s *Server) GetProject(ctx context.Context, req *resourcemanagerpb.GetProjectRequest) (*resourcemanagerpb.Project, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	project, err := s.ProjectStorage.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.GetName(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get project: %v", err)
	}

	return project, nil
}

// ListProjects lists projects under a parent resource.
func (s *Server) ListProjects(ctx context.Context, req *resourcemanagerpb.ListProjectsRequest) (*resourcemanagerpb.ListProjectsResponse, error) {
	if req.GetParent() == "" {
		return nil, status.Error(codes.InvalidArgument, "parent is required")
	}

	pageSize := req.GetPageSize()
	if pageSize <= 0 {
		pageSize = 50
	} else if pageSize > 1000 {
		pageSize = 1000
	}

	listResp, err := s.ProjectStorage.ListResources(ctx, &storage.ListResourcesRequest{
		Parent:    req.GetParent(),
		PageSize:  pageSize,
		PageToken: req.GetPageToken(),
		Filter:    req.GetFilter(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list projects: %v", err)
	}

	return &resourcemanagerpb.ListProjectsResponse{
		Projects:      listResp.Resources,
		NextPageToken: listResp.NextPageToken,
	}, nil
}

// UpdateProject updates an existing project.
func (s *Server) UpdateProject(ctx context.Context, req *resourcemanagerpb.UpdateProjectRequest) (*lropb.Operation, error) {
	projectFromRequest := req.GetProject()
	if projectFromRequest == nil {
		return nil, status.Error(codes.InvalidArgument, "project is required for update")
	}
	if projectFromRequest.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "project.name is required for update")
	}
	updateMask := req.GetUpdateMask()
	if updateMask == nil || len(updateMask.GetPaths()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "update_mask is required")
	}

	updaterFunc := func(existingProject *resourcemanagerpb.Project) (*resourcemanagerpb.Project, error) {
		originalProjectState := proto.Clone(existingProject).(*resourcemanagerpb.Project)

		projectToUpdate := proto.Clone(existingProject).(*resourcemanagerpb.Project)
		fmutils.Overwrite(projectFromRequest, projectToUpdate, updateMask.GetPaths())

		if errs := validation.AssertProjectImmutableFieldsUnchanged(updateMask.GetPaths(), originalProjectState, projectToUpdate); len(errs) > 0 {
			return nil, errs.GRPCStatus().Err()
		}

		if errs := validation.ValidateProject(projectToUpdate); len(errs) > 0 {
			return nil, errs.GRPCStatus().Err()
		}

		projectToUpdate.UpdateTime = timestamppb.Now()
		return projectToUpdate, nil
	}

	if req.GetValidateOnly() {
		existing, err := s.ProjectStorage.GetResource(ctx, &storage.GetResourceRequest{
			Name: projectFromRequest.GetName(),
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get project for validation: %v", err)
		}
		updatedProjectForValidation, err := updaterFunc(existing)
		if err != nil {
			return nil, err
		}
		metadata := &resourcemanagerpb.UpdateProjectMetadata{}
		return longrunning.ResponseOperation(metadata, updatedProjectForValidation, true)
	}

	if req.GetAllowMissing() {
		return nil, status.Error(codes.Unimplemented, "UpdateProject with allow_missing=true is not fully implemented with updater pattern yet")
	}

	updatedProject, err := s.ProjectStorage.UpdateResource(ctx, &storage.UpdateResourceRequest[*resourcemanagerpb.Project]{
		Name:    projectFromRequest.GetName(),
		Updater: updaterFunc,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update project: %v", err)
	}

	metadata := &resourcemanagerpb.UpdateProjectMetadata{}
	return longrunning.ResponseOperation(metadata, updatedProject, true)
}

// DeleteProject deletes a project.
func (s *Server) DeleteProject(ctx context.Context, req *resourcemanagerpb.DeleteProjectRequest) (*lropb.Operation, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required for delete")
	}

	if req.GetValidateOnly() {
		existingProject, err := s.ProjectStorage.GetResource(ctx, &storage.GetResourceRequest{Name: req.GetName()})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get project for delete validation: %v", err)
		}
		metadata := &resourcemanagerpb.DeleteProjectMetadata{}
		return longrunning.ResponseOperation(metadata, existingProject, true)
	}

	deletedProject, err := s.ProjectStorage.DeleteResource(ctx, &storage.DeleteResourceRequest{
		Name: req.GetName(),
		Etag: req.GetEtag(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete project: %v", err)
	}

	metadata := &resourcemanagerpb.DeleteProjectMetadata{}
	var resultForLro proto.Message = deletedProject
	if deletedProject == nil {
		resultForLro = &resourcemanagerpb.Project{}
	}

	return longrunning.ResponseOperation(metadata, resultForLro, true)
}

// MoveProject moves a project to a new parent.
func (s *Server) MoveProject(ctx context.Context, req *resourcemanagerpb.MoveProjectRequest) (*lropb.Operation, error) {
	return nil, status.Errorf(codes.Unimplemented, "method MoveProject not implemented")
}
