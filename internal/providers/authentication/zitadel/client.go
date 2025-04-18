package zitadel

import (
	"context"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"github.com/zitadel/zitadel-go/v3/pkg/client"
	"github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/user/v2"
)

type Zitadel struct {
	Client *client.Client
}

type Config struct {
	Domain   string
	Port     string
	KeyPath  string
	Insecure bool
}

func (z *Zitadel) DeleteUser(ctx context.Context, u *iampb.User) error {
	providerId := u.Annotations[z.GetProviderKey()]

	_, err := z.Client.UserServiceV2().DeleteUser(ctx, &user.DeleteUserRequest{
		UserId: providerId,
	})
	if err != nil {
		return err
	}

	return nil
}

func (z *Zitadel) GetProviderKey() string {
	return "internal.iam.datumapis.com/zitadel-id"
}
