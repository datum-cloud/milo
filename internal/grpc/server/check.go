package server

import (
	"context"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
)

func (s *Server) CheckAccess(ctx context.Context, req *iampb.CheckAccessRequest) (*iampb.CheckAccessResponse, error) {
	return s.AccessChecker(ctx, req)
}
