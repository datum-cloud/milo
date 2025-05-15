package authentication

import (
	"context"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
)

type Provider interface {
	DeleteUser(ctx context.Context, user *iampb.User) error
	GetProviderKey() string
}
