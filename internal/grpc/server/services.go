package server

import (
	"context"
	"fmt"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"github.com/google/uuid"
	"github.com/mennanov/fmutils"
	"go.datum.net/iam/internal/grpc/longrunning"
	"go.datum.net/iam/internal/grpc/validation"
	"go.datum.net/iam/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Server) CreateService(ctx context.Context, req *iampb.CreateServiceRequest) (*longrunningpb.Operation, error) {
	service := req.Service
	service.Name = fmt.Sprintf("services/%s", req.ServiceId)
	service.ServiceId = req.ServiceId

	if errs := validation.ValidateService(service); len(errs) > 0 {
		return nil, errs.GRPCStatus().Err()
	}

	if req.ValidateOnly {
		return longrunning.ResponseOperation(&iampb.CreateServiceMetadata{}, service, true)
	}

	service.Uid = uuid.New().String()
	service.CreateTime = timestamppb.Now()
	service.UpdateTime = timestamppb.Now()

	_, err := s.ServiceStorage.CreateResource(ctx, &storage.CreateResourceRequest[*iampb.Service]{
		Resource: service,
		Name:     service.Name,
	})
	if err != nil {
		return nil, err
	}

	// TODO: Move to temporal or other more robust reconcile workflow
	if err := s.AuthorizationModelReconciler.ReconcileAuthorizationModel(ctx); err != nil {
		return nil, err
	}

	return longrunning.ResponseOperation(&iampb.CreateServiceMetadata{}, service, true)
}

func (s *Server) GetService(ctx context.Context, req *iampb.GetServiceRequest) (*iampb.Service, error) {
	return s.ServiceStorage.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Name,
	})
}

func (s *Server) ListServices(ctx context.Context, req *iampb.ListServicesRequest) (*iampb.ListServicesResponse, error) {
	resources, err := s.ServiceStorage.ListResources(ctx, &storage.ListResourcesRequest{
		PageSize:  req.PageSize,
		PageToken: req.PageToken,
		Filter:    req.Filter,
	})
	if err != nil {
		return nil, err
	}

	return &iampb.ListServicesResponse{
		NextPageToken: resources.NextPageToken,
		Services:      resources.Resources,
	}, nil
}

func (s *Server) UpdateService(ctx context.Context, req *iampb.UpdateServiceRequest) (*longrunningpb.Operation, error) {
	service, err := s.ServiceStorage.UpdateResource(ctx, &storage.UpdateResourceRequest[*iampb.Service]{
		Name: req.Service.Name,
		Updater: func(existing *iampb.Service) (*iampb.Service, error) {
			// Merge in any changes made by the user and run validation.
			fmutils.Overwrite(req.Service, existing, req.GetUpdateMask().GetPaths())

			if errs := validation.ValidateService(existing); len(errs) > 0 {
				return nil, errs.GRPCStatus().Err()
			}
			return existing, nil
		},
	})
	if err != nil {
		return nil, err
	}

	if err := s.AuthorizationModelReconciler.ReconcileAuthorizationModel(ctx); err != nil {
		return nil, err
	}

	return longrunning.ResponseOperation(&iampb.UpdateServiceMetadata{}, service, true)
}

func (s *Server) DeleteService(ctx context.Context, req *iampb.DeleteServiceRequest) (*longrunningpb.Operation, error) {
	service, err := s.ServiceStorage.DeleteResource(ctx, &storage.DeleteResourceRequest{
		Name: req.Name,
	})
	if err != nil {
		return nil, err
	}

	return longrunning.ResponseOperation(&iampb.DeleteServiceMetadata{}, service, true)
}
