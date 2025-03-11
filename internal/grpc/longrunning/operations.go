package longrunning

import (
	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func ResponseOperation(metadata, result proto.Message, done bool) (*longrunningpb.Operation, error) {
	metadataAny, err := anypb.New(metadata)
	if err != nil {
		return nil, err
	}

	resultAny, err := anypb.New(result)
	if err != nil {
		return nil, err
	}

	return &longrunningpb.Operation{
		Name:     "operations/" + uuid.NewString(),
		Done:     done,
		Metadata: metadataAny,
		Result: &longrunningpb.Operation_Response{
			Response: resultAny,
		},
	}, nil
}
