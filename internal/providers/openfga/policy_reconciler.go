package openfga

import (
	"context"
	"encoding/base64"
	"fmt"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"go.datum.net/iam/internal/schema"
	"go.datum.net/iam/internal/subject"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type PolicyReconciler struct {
	StoreID         string
	Client          openfgav1.OpenFGAServiceClient
	SchemaRegistry  *schema.Registry
	SubjectResolver subject.Resolver
}

// ReconcilePolicy will modify an OpenFGA backend to ensure the correct tuples
// exist for the provided IAM policy.
func (r *PolicyReconciler) ReconcilePolicy(ctx context.Context, resource string, policy *iampb.Policy) error {
	resourceReference, err := r.SchemaRegistry.ResolveResource(ctx, resource)
	if err != nil {
		return fmt.Errorf("failed to reconcile policies: %w", err)
	}

	existingTuples, err := r.getExistingPolicyTuples(ctx, resourceReference, policy)
	if err != nil {
		return fmt.Errorf("failed to retrieve existing bindings: %w", err)
	}

	tuples := []*openfgav1.TupleKey{}
	for _, binding := range policy.GetSpec().GetBindings() {
		// Only create tuples to bind the role to the resource when we haven't
		// seen this role binding before.
		tuples = append(
			tuples,
			// Assocates the resource (e.g. project, folder, organization,
			// etc) to the role binding.
			&openfgav1.TupleKey{
				User:     getBindingObjectName(resourceReference.SelfLink, binding.Role),
				Relation: "iam.datumapis.com/RoleBinding",
				Object:   resourceReference.Type + ":" + resourceReference.Name,
			},
			// Associates the role binding to the role that should be bound
			// to the resource.
			&openfgav1.TupleKey{
				User:     "iam.datumapis.com/InternalRole:" + binding.Role,
				Relation: "iam.datumapis.com/InternalRole",
				Object:   getBindingObjectName(resourceReference.SelfLink, binding.Role),
			},
		)

		for _, member := range binding.GetMembers() {
			subjectName, _, err := subject.Parse(member)
			if err != nil {
				return err
			}

			if subjectName != "*" {
				subjectReference, err := r.SubjectResolver(ctx, member)
				if err != nil {
					return fmt.Errorf("failed to resolve member '%s': %w", member, err)
				}

				subjectName = subjectReference.ResourceName
			}

			tuples = append(tuples, &openfgav1.TupleKey{
				User:     "iam.datumapis.com/InternalUser:" + subjectName,
				Relation: "iam.datumapis.com/InternalUser",
				Object:   getBindingObjectName(resourceReference.SelfLink, binding.Role),
			})
		}
	}

	added, removed := diffTuples(existingTuples, tuples)

	writeReq := &openfgav1.WriteRequest{
		StoreId: r.StoreID,
	}

	if len(added) > 0 {
		writeReq.Writes = &openfgav1.WriteRequestWrites{
			TupleKeys: added,
		}
	}

	if len(removed) > 0 {
		writeReq.Deletes = &openfgav1.WriteRequestDeletes{
			TupleKeys: convertTuplesForDelete(removed),
		}
	}

	if writeReq.Deletes == nil && writeReq.Writes == nil {
		return nil
	}

	_, err = r.Client.Write(ctx, writeReq)
	if err != nil {
		return fmt.Errorf("failed to write policy tuples: %w", err)
	}

	return nil
}

func convertTuplesForDelete(tuples []*openfgav1.TupleKey) []*openfgav1.TupleKeyWithoutCondition {
	newTuples := make([]*openfgav1.TupleKeyWithoutCondition, len(tuples))
	for i, tuple := range tuples {
		newTuples[i] = &openfgav1.TupleKeyWithoutCondition{
			User:     tuple.User,
			Relation: tuple.Relation,
			Object:   tuple.Object,
		}
	}
	return newTuples
}

// DiffTuples will return a set of Tuples that were added and a set of Tuples
// that have been removed.
func diffTuples(existing, current []*openfgav1.TupleKey) (added, removed []*openfgav1.TupleKey) {
	// Any of the current tuples that don't exist in the new set of tuples will
	// need to be removed.
	for _, existingTuple := range existing {
		found := false
		for _, currentTuple := range current {
			if cmp.Equal(existingTuple, currentTuple, cmpopts.IgnoreUnexported(openfgav1.TupleKey{})) {
				found = true
				break
			}
		}
		if !found {
			removed = append(removed, existingTuple)
		}
	}

	// Any of the current tuples that don't exist in the new set of tuples will
	// need to be removed.
	for _, currentTuple := range current {
		found := false
		for _, existingTuple := range existing {
			if cmp.Equal(currentTuple, existingTuple, cmpopts.IgnoreUnexported(openfgav1.TupleKey{})) {
				found = true
				break
			}
		}
		if !found {
			added = append(added, currentTuple)
		}
	}
	return added, removed
}

func (r *PolicyReconciler) getExistingPolicyTuples(ctx context.Context, resource *schema.ResourceReference, policy *iampb.Policy) ([]*openfgav1.TupleKey, error) {
	tuples, err := getTupleKeys(ctx, r.StoreID, r.Client, &openfgav1.ReadRequestTupleKey{
		Relation: "iam.datumapis.com/RoleBinding",
		Object:   resource.Type + ":" + resource.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get policy tuples: %w", err)
	}

	for _, binding := range policy.GetSpec().GetBindings() {
		bindingTuples, err := getTupleKeys(ctx, r.StoreID, r.Client, &openfgav1.ReadRequestTupleKey{
			Object: getBindingObjectName(resource.SelfLink, binding.Role),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get binding tuples: %w", err)
		}
		tuples = append(tuples, bindingTuples...)
	}
	return tuples, nil
}

func getBindingObjectName(resource, role string) string {
	roleBindingHash := base64.RawStdEncoding.EncodeToString([]byte(resource + role))
	return "iam.datumapis.com/RoleBinding:" + roleBindingHash
}

func getTupleKeys(ctx context.Context, storeID string, client openfgav1.OpenFGAServiceClient, tuple *openfgav1.ReadRequestTupleKey) ([]*openfgav1.TupleKey, error) {
	tupleKeys := []*openfgav1.TupleKey{}
	continuationToken := ""
	for {
		resp, err := client.Read(ctx, &openfgav1.ReadRequest{
			StoreId:           storeID,
			ContinuationToken: continuationToken,
			PageSize:          wrapperspb.Int32(100),
			TupleKey:          tuple,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to read existing tuples: %w", err)
		}

		for _, tuple := range resp.Tuples {
			tupleKeys = append(tupleKeys, tuple.GetKey())
		}

		continuationToken = resp.ContinuationToken
		if resp.ContinuationToken == "" {
			break
		}
	}

	return tupleKeys, nil
}
