package server

import (
	"context"
	"fmt"

	resourcemanagerpb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/resourcemanager/v1alpha"
	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"github.com/google/uuid"
	"github.com/mennanov/fmutils"
	"go.datum.net/iam/internal/grpc/longrunning"
	"go.datum.net/iam/internal/grpc/validation"
	"go.datum.net/iam/internal/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateProject creates a new project resource.
func (s *Server) CreateProject(ctx context.Context, req *resourcemanagerpb.CreateProjectRequest) (*longrunningpb.Operation, error) {
	project := proto.Clone(req.GetProject()).(*resourcemanagerpb.Project)
	project.Uid = uuid.New().String()
	if req.GetProjectId() != "" {
		project.ProjectId = req.GetProjectId()
	} else {
		project.ProjectId = project.Uid
	}

	if errs := validation.ValidateProject(req.GetProject()); len(errs) > 0 {
		return nil, errs.GRPCStatus().Err()
	}

	if req.GetValidateOnly() {
		metadata := &resourcemanagerpb.CreateProjectMetadata{}
		return longrunning.ResponseOperation(metadata, req.GetProject(), true)
	}

	project.Name = fmt.Sprintf("projects/%s", project.ProjectId)
	project.Parent = req.GetParent()

	now := timestamppb.Now()
	project.CreateTime = now
	project.UpdateTime = now

	createdProject, err := s.ProjectStorage.CreateResource(ctx, &storage.CreateResourceRequest[*resourcemanagerpb.Project]{
		Parent:   req.GetParent(),
		Resource: project,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create project: %v", err)
	}

	metadata := &resourcemanagerpb.CreateProjectMetadata{}
	return longrunning.ResponseOperation(metadata, createdProject, true)
}

// GetProject retrieves a project resource by its name.
func (s *Server) GetProject(ctx context.Context, req *resourcemanagerpb.GetProjectRequest) (*resourcemanagerpb.Project, error) {
	return s.ProjectStorage.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.GetName(),
	})
}

// ListProjects lists projects under a parent resource.
func (s *Server) ListProjects(ctx context.Context, req *resourcemanagerpb.ListProjectsRequest) (*resourcemanagerpb.ListProjectsResponse, error) {
	listResp, err := s.ProjectStorage.ListResources(ctx, &storage.ListResourcesRequest{
		Parent:    req.GetParent(),
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
		Filter:    req.GetFilter(),
	})
	if err != nil {
		return nil, err
	}

	return &resourcemanagerpb.ListProjectsResponse{
		Projects:      listResp.Resources,
		NextPageToken: listResp.NextPageToken,
	}, nil
}

// UpdateProject updates an existing project.
func (s *Server) UpdateProject(ctx context.Context, req *resourcemanagerpb.UpdateProjectRequest) (*longrunningpb.Operation, error) {
	projectFromRequest := req.GetProject()
	if projectFromRequest == nil {
		return nil, status.Error(codes.InvalidArgument, "project is required for update")
	}
	if projectFromRequest.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "project.name is required for update")
	}

	updaterFunc := func(existingProject *resourcemanagerpb.Project) (*resourcemanagerpb.Project, error) {
		projectToUpdate := proto.Clone(existingProject).(*resourcemanagerpb.Project)
		fmutils.Overwrite(projectFromRequest, projectToUpdate, req.GetUpdateMask().GetPaths())

		if errs := validation.ValidateProject(projectToUpdate); len(errs) > 0 {
			return nil, errs.GRPCStatus().Err()
		}

		projectToUpdate.UpdateTime = timestamppb.Now()
		return projectToUpdate, nil
	}

	// TODO: Implement allow_missing=true
	if req.GetAllowMissing() {
		return nil, status.Errorf(codes.InvalidArgument, "allow_missing is not supported for UpdateProject")
	}

	existing, err := s.ProjectStorage.GetResource(ctx, &storage.GetResourceRequest{
		Name: projectFromRequest.GetName(),
	})
	if err != nil {
		return nil, err
	}
	updatedProjectForValidation, err := updaterFunc(existing)
	if err != nil {
		return nil, err
	}

	if req.GetValidateOnly() {
		metadata := &resourcemanagerpb.UpdateProjectMetadata{}
		return longrunning.ResponseOperation(metadata, updatedProjectForValidation, true)
	}

	updatedProject, err := s.ProjectStorage.UpdateResource(ctx, &storage.UpdateResourceRequest[*resourcemanagerpb.Project]{
		Name:    projectFromRequest.GetName(),
		Updater: updaterFunc,
	})
	if err != nil {
		return nil, err
	}

	metadata := &resourcemanagerpb.UpdateProjectMetadata{}
	return longrunning.ResponseOperation(metadata, updatedProject, true)
}

// DeleteProject deletes a project.
func (s *Server) DeleteProject(ctx context.Context, req *resourcemanagerpb.DeleteProjectRequest) (*longrunningpb.Operation, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required for delete")
	}

	if req.GetValidateOnly() {
		existingProject, err := s.ProjectStorage.GetResource(ctx, &storage.GetResourceRequest{Name: req.GetName()})
		if err != nil {
			return nil, err
		}
		metadata := &resourcemanagerpb.DeleteProjectMetadata{}
		return longrunning.ResponseOperation(metadata, existingProject, true)
	}

	deletedProject, err := s.ProjectStorage.DeleteResource(ctx, &storage.DeleteResourceRequest{
		Name: req.GetName(),
		Etag: req.GetEtag(),
	})
	if err != nil {
		return nil, err
	}

	metadata := &resourcemanagerpb.DeleteProjectMetadata{}
	return longrunning.ResponseOperation(metadata, deletedProject, true)
}

// MoveProject moves a project to a new parent.
func (s *Server) MoveProject(ctx context.Context, req *resourcemanagerpb.MoveProjectRequest) (*longrunningpb.Operation, error) {
	return nil, status.Errorf(codes.Unimplemented, "MoveProject is not implemented")
}
