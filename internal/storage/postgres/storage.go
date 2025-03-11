// The postgres package provides an implementation of the resource storage
// interface that's compatible with a postgres storage backend.
package postgres

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"go.datum.net/iam/internal/storage"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// databaseStorage provides an implementation of the resource storage interface
// that stores resource in a postgres database. Resources will be stored as a
// protojson encoded format in a JSONB column.
type databaseStorage[T storage.Resource] struct {
	database *sql.DB

	// Represents a zero value of the underlying type being stored. This is here
	// for convinence when returning the zero value from a function when an error
	// is encountered. The zero value should never be used in any non-error
	// scenario.
	zero T
}

// GetResource will retrieve the resource from the underlying postgres database
// table. An gRPC NotFound error will be returned if the requested resource does
// not exist.
func (r *databaseStorage[T]) GetResource(ctx context.Context, req *storage.GetResourceRequest) (T, error) {
	return r.getResource(ctx, r.database, req)
}

// ListResources will list a page of resources from the underlying postgres
// database.
//
// TODO: Support pagination.
func (r *databaseStorage[T]) ListResources(ctx context.Context, req *storage.ListResourcesRequest) (*storage.ListResourcesResponse[T], error) {
	// Set the default page size when not provided.
	if req.PageSize == 0 {
		req.PageSize = 50
	}

	pageInfo, err := getPageToken(req)
	if err != nil {
		return nil, err
	}

	// Pull the resources from the database.
	statement, err := r.database.PrepareContext(
		ctx,
		fmt.Sprintf(
			"SELECT uid, name, parent, data FROM %s WHERE parent = $1 AND data->'deleteTime' IS NULL LIMIT %d OFFSET %d",
			resourceTableName(r.zero),
			pageInfo.PageSize,
			pageInfo.PageSize*(pageInfo.PageNumber-1),
		),
	)
	if err != nil {
		return nil, err
	}
	res, err := statement.QueryContext(ctx, req.Parent)
	if err != nil {
		return nil, err
	}

	var resources []T
	// Verify we actually got a result from the database
	for res.Next() {
		resource, err := r.scanResource(res)
		if err != nil {
			return nil, err
		}

		resources = append(resources, resource)
	}

	var nextPageToken string
	// Assume that if we were able to retrieve the number of resources that were
	// requested then there's another page of resources available.
	if len(resources) == int(pageInfo.PageSize) {
		nextPageToken, err = encodePageToken(pageToken{
			PageNumber: pageInfo.PageNumber + 1,
			PageSize:   pageInfo.PageSize,
			Filter:     pageInfo.Filter,
		})
		if err != nil {
			return nil, err
		}
	}

	return &storage.ListResourcesResponse[T]{
		Resources:     resources,
		NextPageToken: nextPageToken,
	}, nil
}

// CreateResource will create a new resource in the underlying storage
// implementation.
func (r *databaseStorage[T]) CreateResource(ctx context.Context, req *storage.CreateResourceRequest[T]) (T, error) {
	resource := proto.Clone(req.Resource).(T)
	resourceReflector := resource.ProtoReflect()
	resourceFields := resourceReflector.Descriptor().Fields()

	// Set the UID of the resource when the user hasn't provided one. This is
	// there for convenience so callers don't always have to create a new UUID.
	uidField := resourceFields.ByName("uid")
	if resourceReflector.Get(uidField).String() == "" {
		resource.ProtoReflect().Set(uidField, protoreflect.ValueOfString(uuid.NewString()))
	}

	// Convert the resource into an Any type so we can store
	// it in the database with it's type information
	anyResource, err := anypb.New(resource)
	if err != nil {
		return r.zero, err
	}

	// Convert the cloned resource to json that can be stored in the database.
	reqJson, err := protojson.Marshal(anyResource)
	if err != nil {
		return r.zero, err
	}

	// Start a database transactions to ensure that the resource can be
	// created atomically.
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return r.zero, err
	}

	// Verify that a resource with the same name doesn't already exist.
	_, err = r.getResource(ctx, tx, &storage.GetResourceRequest{
		Name: req.Name,
	})
	if err != nil && status.Code(err) != codes.NotFound {
		return r.zero, err
	} else if err == nil {
		return r.zero, status.Error(codes.AlreadyExists, "Resource already exists")
	}

	// Prepare the database query to insert the resource into the database.
	statement, err := tx.PrepareContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (uid, name, parent, data) VALUES ($1, $2, $3, $4)",
		resourceTableName(resource),
	))
	if err != nil {
		return r.zero, err
	}

	// Insert the resource into the database
	res, err := statement.ExecContext(
		ctx,
		resource.GetUid(),
		resource.GetName(),
		req.Parent,
		reqJson,
	)
	if err != nil {
		return r.zero, err
	}

	if _, err := res.RowsAffected(); err != nil {
		return r.zero, err
	}

	if err := tx.Commit(); err != nil {
		return r.zero, err
	}

	return resource, nil
}

// UpdateResource will update an existing resource in the underlying storage
// database.
func (r *databaseStorage[T]) UpdateResource(ctx context.Context, req *storage.UpdateResourceRequest[T]) (T, error) {
	return r.atomicUpdateResource(ctx, req.Name, req.Updater)
}

// DeleteResource will soft-delete a resource from the underlying database. The
// etag of the resource can be provided to ensure that the latest version of the
// resource was retrieved before deletion.
//
// TODO: Enforce etag validation.
func (r *databaseStorage[T]) DeleteResource(ctx context.Context, req *storage.DeleteResourceRequest) (T, error) {
	// Atomically set the deletion timestamp of the resource.
	return r.atomicUpdateResource(ctx, req.Name, func(existing T) (T, error) {
		// // Set the deletion timestamp on the resource
		existing.ProtoReflect().Set(
			// Assume that the resource has a delete_time field defined.
			existing.ProtoReflect().Descriptor().Fields().ByName("delete_time"),
			// Set to the current timestamp.
			protoreflect.ValueOfMessage(timestamppb.Now().ProtoReflect()),
		)
		return existing, nil
	})
}

func (r *databaseStorage[T]) UndeleteResource(ctx context.Context, req *storage.UndeleteResourceRequest) (T, error) {
	return r.atomicUpdateResource(ctx, req.Name, func(existing T) (T, error) {
		clone := proto.Clone(existing).(T)
		clone.ProtoReflect().Clear(clone.ProtoReflect().Descriptor().Fields().ByName("delete_time"))
		return clone, nil
	})
}

func (r *databaseStorage[T]) PurgeResource(ctx context.Context, req *storage.PurgeResourceRequest) (resource T, err error) {
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return r.zero, err
	}

	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				err = fmt.Errorf("failed to rollback transaction: %s: %w", rollbackErr, err)
			}
		} else {
			err = tx.Commit()
		}
	}()

	resource, err = r.getResource(ctx, tx, &storage.GetResourceRequest{
		Name: req.Name,
	})
	if err != nil {
		return r.zero, err
	}

	stmt, err := tx.PrepareContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE name = $1", resourceTableName(r.zero)))
	if err != nil {
		return r.zero, err
	}

	_, err = stmt.ExecContext(ctx, resource.GetName())
	if err != nil {
		return r.zero, err
	}

	return resource, nil
}

// Defines a new interface that represents a database being created.
type database interface {
	PrepareContext(context.Context, string) (*sql.Stmt, error)
}

// getResource supports retrieving the resource from the underlying database as
// part of a database transaction if necessary so an atomic update can be
// performed.
func (r *databaseStorage[T]) getResource(ctx context.Context, database database, req *storage.GetResourceRequest) (T, error) {
	statement, err := database.PrepareContext(
		ctx,
		fmt.Sprintf(
			"SELECT uid, name, parent, data FROM %s WHERE name = $1",
			resourceTableName(r.zero),
		),
	)
	if err != nil {
		return r.zero, err
	}
	defer statement.Close()

	res, err := statement.QueryContext(ctx, req.Name)
	if err != nil {
		return r.zero, err
	}
	defer res.Close()

	// Verify we actually got a result from the database
	if !res.Next() {
		return r.zero, status.Error(codes.NotFound, "resource not found")
	}

	// Pull the resource from the database
	return r.scanResource(res)
}

// scanResource will retrieve the underlying resource from the postgres database
// table. The resource is expected to be stored as a protobuf.Any resource with
// the type information embedded.
func (r *databaseStorage[T]) scanResource(
	scanner interface {
		Scan(dest ...interface{}) error
	},
) (T, error) {
	var uid, name, parent, data string
	if err := scanner.Scan(&uid, &name, &parent, &data); err != nil {
		return r.zero, fmt.Errorf("failed to scan existing resource: %w", err)
	}

	// Create a new any type to unmarshal the resource into
	anyResource := &anypb.Any{}
	if err := protojson.Unmarshal([]byte(data), anyResource); err != nil {
		return r.zero, fmt.Errorf("failed to parse existing resource: %w", err)
	}

	resource, err := anyResource.UnmarshalNew()
	if err != nil {
		return r.zero, err
	}

	// Create a reflection and a new instance of the message
	resourceReflector := resource.ProtoReflect()
	resourceFields := resourceReflector.Descriptor().Fields()

	// Set the fields the server is responsible for settings
	resourceReflector.Set(resourceFields.ByName("uid"), protoreflect.ValueOfString(uid))
	resourceReflector.Set(resourceFields.ByName("name"), protoreflect.ValueOfString(name))

	return resource.(T), nil
}

// Updater func provides an interface that can be used when doing an atomic update
// to a resource. A new instance of the resource should be returned for storage.
// Any fields marked as IMMUTABLE will be overwritten with the existing entry's
// value.
//
// TODO: Add feature for IMMUTABLE check
type updaterFunc[T proto.Message] func(existing T) (T, error)

// This function will retrieve a resource from the database for updating using the
// provided function. This function can gurantee that no other updates can be made
// to the resource while this update is running. An Aborted error will be returned
// during conflicts. The existing resource will be unmarshalled into its base type.
func (r *databaseStorage[T]) atomicUpdateResource(ctx context.Context, resourceName string, updater updaterFunc[T]) (T, error) {
	// Start a database transaction so we can atomically update the resource.
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return r.zero, err
	}

	// Grab the existing resource from the database. This is run
	// in the transaction and will hold a lock.
	existingResource, err := r.getResource(ctx, tx, &storage.GetResourceRequest{
		Name: resourceName,
	})
	if err != nil {
		return r.zero, err
	}

	// Pass a clone of the existing resource in so we can confirm any changes made
	// in the updater function won't conflict.
	updatedResource, err := updater(proto.Clone(existingResource).(T))
	if err != nil {
		return r.zero, err
	}

	// Verify that the updates do not conflict.
	if existingResource.GetEtag() != updatedResource.GetEtag() {
		// Inform the user there was a conflict and they have to try again.
		return r.zero, status.Error(codes.Aborted, "resource %q has been modified. please apply your changes to the latest version and try again")
	}

	// Grab the reflection of the resource for reference to later
	resourceFields := existingResource.ProtoReflect().Descriptor().Fields()

	// Set the unique ID of the resource message before it's stored in the database.
	updatedResource.ProtoReflect().Set(resourceFields.ByName("update_time"), protoreflect.ValueOfMessage(timestamppb.Now().ProtoReflect()))

	// Convert the resource into an Any type so we can store the protobuf message
	// in the database with it's type information.
	anyResource, err := anypb.New(updatedResource)
	if err != nil {
		return r.zero, err
	}

	reqJson, err := protojson.Marshal(anyResource)
	if err != nil {
		return r.zero, err
	}

	statement, err := tx.PrepareContext(ctx, fmt.Sprintf(
		"UPDATE %s SET data = $1 WHERE name = $2",
		resourceTableName(existingResource),
	))
	if err != nil {
		return r.zero, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer statement.Close()

	updateRes, err := statement.ExecContext(
		ctx,
		reqJson,
		updatedResource.GetName(),
	)
	if err != nil {
		return r.zero, fmt.Errorf("failed to update resource: %w", err)
	}

	if _, err := updateRes.RowsAffected(); err != nil {
		return r.zero, fmt.Errorf("failed to retrieve rows affected: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return r.zero, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return updatedResource, nil
}

// resourceTableName will retrieve the name of the database table that is used
// to store the records for the underlying resource.
func resourceTableName(resource storage.Resource) string {
	resourceDescriptor := proto.GetExtension(resource.ProtoReflect().Descriptor().Options(), annotations.E_Resource).(*annotations.ResourceDescriptor)

	return fmt.Sprintf(
		"%s_resource",
		regexp.MustCompile("[./]").ReplaceAllString(resourceDescriptor.Type, "_"),
	)
}

type pageToken struct {
	PageNumber int32
	PageSize   int32
	Filter     string
}

func getPageToken(req *storage.ListResourcesRequest) (pageToken, error) {
	pageInfo := &pageToken{
		PageSize:   req.PageSize,
		PageNumber: 1,
		Filter:     req.Filter,
	}

	if req.PageToken != "" {
		decodedToken, err := base64.StdEncoding.DecodeString(req.PageToken)
		if err != nil {
			return pageToken{}, status.Error(codes.InvalidArgument, "invalid page token provided")
		}

		if err := json.Unmarshal(decodedToken, pageInfo); err != nil {
			return pageToken{}, status.Error(codes.InvalidArgument, "invalid page token provided")
		}
	}

	return *pageInfo, nil
}

func encodePageToken(token pageToken) (string, error) {
	encodedToken, err := json.Marshal(token)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(encodedToken), nil
}
