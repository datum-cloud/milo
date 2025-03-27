package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	annotations "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/api"
	"buf.build/gen/go/datum-cloud/iam/grpc/go/datum/iam/v1alpha/iamv1alphagrpc"
	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"go.datum.net/iam/internal/grpc/errors"
	"go.datum.net/iam/internal/storage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	googleannotations "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/genproto/googleapis/cloud/audit"
	"google.golang.org/genproto/googleapis/rpc/context/attribute_context"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// A subject extractor will extract the Auth subject from
type SubjectExtractor func(context.Context) (string, error)

// ResourceResolver is used by the IAMInterceptor to resolve the resource URL
// that is used when checking for the authenticated user's access.
type ResourceResolver func(method protoreflect.MethodDescriptor, request proto.Message) (string, string, error)

// IAMInterceptor performs authorization checks to confirm authenticated clients
// have access to perform actions against resources based on IAM Policies
// created on the platform.
//
// gRPC methods can use the `datum.api.required_permissions` protobuf annotation
// to configure which permissions to check against the resource that is being
// acted upon.
//
// ```
//
//	option (datum.api.required_permissions) = "iam.datumapis.com/serviceAccounts.update";
//
// ```
//
// The subject extractor will determine which subject is authenticated for the
// gRPC request. The SubjectExtractor is expected to return an Unauthenticated
// error if the client doesn't provide valid authentication information.
//
// The resource resolver is used to determine which resource on the platform the
// gRPC method is acting against. The parent resolver is used to build the
// resource hierarchy in the platform so access can be inherited from parent
// resources.
func SubjectAuthorizationInterceptor(
	accessChecker iamv1alphagrpc.AccessCheckClient,
	subjectExtractor SubjectExtractor,
	resourceResolver ResourceResolver,
	parentResolver storage.ParentResolver,
) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		parts := strings.Split(info.FullMethod, "/")
		slog.InfoContext(ctx, "processing IAM Check for gRPC method", slog.String("grpc_service", parts[1]))

		serviceDescriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(parts[1]))
		if err != nil {
			return nil, err
		}

		// Grab the descriptor for the RPC method that's being called
		methodDescriptor := serviceDescriptor.(protoreflect.ServiceDescriptor).Methods().ByName(protoreflect.Name(parts[2]))

		var requiredPermissions []string
		// Grab the required permissions for the endpoint
		if proto.HasExtension(methodDescriptor.Options(), annotations.E_RequiredPermissions) {
			requiredPermissions = proto.GetExtension(methodDescriptor.Options(), annotations.E_RequiredPermissions).([]string)
		} else {
			slog.WarnContext(ctx, "gRPC method does not have 'datum.api.required_permissions' annotation", slog.String("grpc_method", info.FullMethod))
			return handler(ctx, req)
		}

		checkAccessCtx, span := otel.Tracer("").Start(ctx, "datum.auth.CheckAccess", trace.WithAttributes(
			attribute.String("permission", requiredPermissions[0]),
		))

		subject, err := subjectExtractor(checkAccessCtx)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.End()
			return nil, err
		}

		span.SetAttributes(attribute.String("subject", subject))

		resourceURL, resourceType, err := resourceResolver(methodDescriptor, req.(proto.Message))
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.End()
			return nil, err
		}

		span.SetAttributes(attribute.String("resource", resourceURL))

		parents, err := resolveParents(checkAccessCtx, parentResolver, &storage.ResourceReference{
			Name:     storage.ResourceName(resourceURL),
			Type:     resourceType,
			SelfLink: fmt.Sprintf("%s/%s", storage.ServiceName(resourceType), storage.ResourceName(resourceURL)),
		})
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.End()
			return nil, err
		}

		var accessCheckContext []*iampb.CheckContext
		for _, parent := range parents {
			accessCheckContext = append(accessCheckContext, &iampb.CheckContext{
				ContextType: &iampb.CheckContext_ParentRelationship{
					ParentRelationship: parent,
				},
			})
		}

		resp, err := accessChecker.CheckAccess(checkAccessCtx, &iampb.CheckAccessRequest{
			Subject:    subject,
			Permission: requiredPermissions[0],
			Resource:   resourceURL,
			Context:    accessCheckContext,
		})
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.End()
			return nil, err
		}

		auditLog := &audit.AuditLog{
			AuthorizationInfo: []*audit.AuthorizationInfo{{
				Resource:   resourceURL,
				Permission: requiredPermissions[0],
				Granted:    resp.Allowed,
				ResourceAttributes: &attribute_context.AttributeContext_Resource{
					// TODO: Fill out this section
					Service: "",
				},
			}},
			AuthenticationInfo: &audit.AuthenticationInfo{
				PrincipalEmail:   subject,
				PrincipalSubject: "user:" + subject,
			},
			MethodName:   string(methodDescriptor.Name()),
			ServiceName:  string(serviceDescriptor.Name()),
			ResourceName: resourceURL,
		}

		slog.InfoContext(
			checkAccessCtx,
			string(auditLog.ProtoReflect().Descriptor().FullName()),
			slog.Any(string(auditLog.ProtoReflect().Descriptor().FullName()), auditLog),
		)
		span.End()

		if !resp.Allowed {
			return "", errors.PermissionDenied().Err()
		}

		return handler(ctx, req)
	}
}

func resolveParents(ctx context.Context, parentResolver storage.ParentResolver, resource *storage.ResourceReference) ([]*iampb.ParentRelationship, error) {
	var parents []*iampb.ParentRelationship
	for {
		parent, err := parentResolver.ResolveParent(ctx, resource)
		if err != nil {
			return nil, err
		} else if parent == nil {
			// Resource does not have a parent so we can skip looking for additional
			// parents.
			break
		}

		parents = append(parents, &iampb.ParentRelationship{
			ParentResource: parent.SelfLink,
			ChildResource:  resource.SelfLink,
		})

		// Set the resource as the parent so we continue trying to find parent
		// resources.
		resource = parent
	}

	return parents, nil
}

// ResourceNameResolver will attempt to resolve the resource name in a gRPC
// request that can be used to check a user's access.
func ResourceNameResolver() ResourceResolver {
	return func(method protoreflect.MethodDescriptor, req proto.Message) (string, string, error) {
		message := req.ProtoReflect()
		descriptor := message.Descriptor()
		fields := descriptor.Fields()

		if proto.HasExtension(method.Options(), annotations.E_IamResourceName) {
			resourceName := proto.GetExtension(method.Options(), annotations.E_IamResourceName).(string)
			if resourceName != "" {
				return resourceName, "", nil
			}
		}

		// Messages must meet one of the following criteria to be supported by this authorization interceptor:
		//   * Message MUST define a "parent" field and MAY provide a value
		//   * Message MUST define a "name" field and MUST provide a value
		//   * Message MUST define a "resource" field and MUST provide a value
		//   * Message MUST embed a message with the `google.api.resource` option with a `name` field and MUST provide a value.
		var resourceNameField protoreflect.FieldDescriptor
		if name := fields.ByName("name"); name != nil {
			resourceNameField = name
		} else if parent := fields.ByName("parent"); parent != nil {
			resourceNameField = parent
		} else if resource := fields.ByName("resource"); resource != nil {
			resourceNameField = resource
		}

		if resourceNameField != nil {
			resourceType := resourceReferenceType(resourceNameField)
			resourceName := message.Get(resourceNameField).String()

			return storage.ServiceName(resourceType) + "/" + resourceName, resourceType, nil
		}

		// Loop through each of the fields that are defined on the protobuf file
		// and try to find a resource with the `google.api.resource` annotation.
		for i := 0; i < fields.Len(); i++ {
			field := fields.Get(i)

			// The `google.api.resource` annotation is only set on message types.
			if field.Message() == nil {
				continue
			}

			fieldValue := message.Get(field)
			if !fieldValue.IsValid() {
				continue
			}

			embdededMessage := fieldValue.Message()
			embeddedDescriptor := embdededMessage.Descriptor()

			if !proto.HasExtension(embeddedDescriptor.Options(), googleannotations.E_Resource) {
				continue
			}

			resourceDescriptor := proto.GetExtension(embeddedDescriptor.Options(), googleannotations.E_Resource).(*googleannotations.ResourceDescriptor)

			return storage.ServiceName(resourceDescriptor.Type) + "/" + embdededMessage.Get(embeddedDescriptor.Fields().ByName("name")).String(), resourceDescriptor.Type, nil
		}

		return "", "", fmt.Errorf("failed to resolve resource name")
	}
}

func resourceReferenceType(field protoreflect.FieldDescriptor) string {
	return proto.GetExtension(field.Options(), googleannotations.E_ResourceReference).(*googleannotations.ResourceReference).Type
}
