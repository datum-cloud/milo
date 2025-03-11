package errors

import (
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/protoadapt"
)

func New(code codes.Code, msg string, details ...protoadapt.MessageV1) *status.Status {
	s, err := status.New(code, msg).WithDetails(details...)
	if err != nil {
		return status.New(codes.Internal, "internal error")
	}

	return s
}

func Unauthenticated() *status.Status {
	return New(codes.Unauthenticated, "Request is unauthenticated. Please provide an authentication token and try again.", &errdetails.Help{
		Links: []*errdetails.Help_Link{{
			Description: "Authentication Guide",
			Url:         "https://docs.datum.net/api/guides/authentication",
		}},
	})
}

// PermissionDenied returnes the standard error message that is always used when
// a user does not have access to a resource they're acting on.
func PermissionDenied() *status.Status {
	return New(codes.PermissionDenied, "Permission denied, or resource was not found", &errdetails.Help{
		Links: []*errdetails.Help_Link{{
			Description: "Errors Guide",
			Url:         "https://docs.datum.net/api/guides/errors/permission_denied",
		}},
	})
}

func Internal() *status.Status {
	return New(
		codes.Internal,
		"Internal error encountered. Please reach out to support for additional help with the request. Please provide the request ID in the error details.",
		&errdetails.ErrorInfo{
			Domain: "service.datumapis.com",
			Reason: "InternalError",
		},
		&errdetails.Help{
			Links: []*errdetails.Help_Link{{
				Description: "Support Portal",
				Url:         "https://support.datum.net",
			}},
		},
	)
}
