package server_test

import (
	"buf.build/gen/go/datum-cloud/iam/grpc/go/datum/iam/v1alpha/iamv1alphagrpc"
	"go.datum.net/iam/internal/grpc/server"
)

var _ iamv1alphagrpc.AccessCheckServer = &server.Server{}
var _ iamv1alphagrpc.IAMPolicyServer = &server.Server{}
var _ iamv1alphagrpc.RolesServer = &server.Server{}
var _ iamv1alphagrpc.ServicesServer = &server.Server{}
