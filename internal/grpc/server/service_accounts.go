package server

import (
	"context"
	"fmt"
	"strings"

	resourcemanagerpb "buf.build/gen/go/datum-cloud/datum-os/protocolbuffers/go/datum/os/resourcemanager/v1alpha"
	"buf.build/gen/go/datum-cloud/iam/grpc/go/datum/iam/v1alpha/iamv1alphagrpc"
	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	hydra "github.com/ory/hydra-client-go/v2"
	"go.datum.net/iam/internal/storage"
	"google.golang.org/protobuf/proto"
)

type ServiceAccountServerOptions struct {
	ServiceAccountKeys storage.ResourceServer[*iampb.ServiceAccountKey]
	ServiceAccounts    storage.ResourceServer[*iampb.ServiceAccount]
	Projects           storage.ResourceGetter[*resourcemanagerpb.Project]
	HydraClient        hydra.OAuth2API
}

func NewServiceAccountServer(opts *ServiceAccountServerOptions) iamv1alphagrpc.ServiceAccountsServer {
	return &serviceAccounts{
		hydraClient:     opts.HydraClient,
		keys:            opts.ServiceAccountKeys,
		serviceAccounts: opts.ServiceAccounts,
		projects:        opts.Projects,
	}
}

type serviceAccounts struct {
	iamv1alphagrpc.UnimplementedServiceAccountsServer

	keys            storage.ResourceServer[*iampb.ServiceAccountKey]
	serviceAccounts storage.ResourceServer[*iampb.ServiceAccount]
	projects        storage.ResourceGetter[*resourcemanagerpb.Project]

	hydraClient hydra.OAuth2API
}

func (s *serviceAccounts) ListServiceAccounts(ctx context.Context, req *iampb.ListServiceAccountsRequest) (*iampb.ListServiceAccountsResponse, error) {
	_, err := s.projects.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Parent,
	})
	if err != nil {
		return nil, err
	}

	listResp, err := s.serviceAccounts.ListResources(ctx, &storage.ListResourcesRequest{
		Parent:    req.Parent,
		PageSize:  req.PageSize,
		PageToken: req.PageToken,
		Filter:    req.Filter,
	})
	if err != nil {
		return nil, err
	}

	return &iampb.ListServiceAccountsResponse{
		ServiceAccounts: listResp.Resources,
		NextPageToken:   listResp.NextPageToken,
	}, nil
}

func (s *serviceAccounts) GetServiceAccount(ctx context.Context, req *iampb.GetServiceAccountRequest) (*iampb.ServiceAccount, error) {
	return s.serviceAccounts.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Name,
	})
}

func (s *serviceAccounts) CreateServiceAccount(ctx context.Context, req *iampb.CreateServiceAccountRequest) (*iampb.ServiceAccount, error) {
	project, err := s.projects.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Parent,
	})
	if err != nil {
		return nil, err
	}

	serviceAccount := proto.Clone(req.ServiceAccount).(*iampb.ServiceAccount)
	serviceAccount.ServiceAccountId = fmt.Sprintf("%s@%s.iam.datumapis.com", req.ServiceAccountId, strings.TrimPrefix(project.Name, "projects/"))
	serviceAccount.Parent = project.Name
	serviceAccount.Name = fmt.Sprintf("%s/serviceAccounts/%s", serviceAccount.Parent, serviceAccount.ServiceAccountId)

	createdServiceAccount, err := s.serviceAccounts.CreateResource(ctx, &storage.CreateResourceRequest[*iampb.ServiceAccount]{
		Name:     serviceAccount.Name,
		Parent:   serviceAccount.Parent,
		Resource: serviceAccount,
	})
	if err != nil {
		return nil, err
	}
	return createdServiceAccount, nil
}
