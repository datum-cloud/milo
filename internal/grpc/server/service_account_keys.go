package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"github.com/google/uuid"
	hydra "github.com/ory/hydra-client-go/v2"
	"go.datum.net/iam/internal/storage"
	"google.golang.org/protobuf/proto"
)

func (s *serviceAccounts) ListServiceAccountKeys(ctx context.Context, req *iampb.ListServiceAccountKeysRequest) (*iampb.ListServiceAccountKeysResponse, error) {
	listResp, err := s.keys.ListResources(ctx, &storage.ListResourcesRequest{
		Parent:    req.Parent,
		PageSize:  req.PageSize,
		PageToken: req.PageToken,
		Filter:    req.Filter,
	})
	if err != nil {
		return nil, err
	}

	return &iampb.ListServiceAccountKeysResponse{
		ServiceAccountKeys: listResp.Resources,
		NextPageToken:      listResp.NextPageToken,
	}, nil
}

func (s *serviceAccounts) GetServiceAccountKey(ctx context.Context, req *iampb.GetServiceAccountKeyRequest) (*iampb.ServiceAccountKey, error) {
	return s.keys.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Name,
	})
}

func (s *serviceAccounts) CreateServiceAccountKey(ctx context.Context, req *iampb.CreateServiceAccountKeyRequest) (*iampb.ServiceAccountKey, error) {
	serviceAccount, err := s.serviceAccounts.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Parent,
	})
	if err != nil {
		return nil, err
	}

	privateKey, publicKey, err := createECDSAKeyPair()
	if err != nil {
		return nil, err
	}

	encodedPrivateKey := x509.MarshalPKCS1PrivateKey(privateKey)

	pemPrivateKey := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: encodedPrivateKey,
	}

	serviceAccountKey := proto.Clone(req.ServiceAccountKey).(*iampb.ServiceAccountKey)
	serviceAccountKey.Uid = uuid.NewString()
	serviceAccountKey.ServiceAccountKeyId = serviceAccountKey.Uid
	serviceAccountKey.Name = fmt.Sprintf("%s/keys/%s", req.Parent, serviceAccountKey.Uid)
	serviceAccountKey.Parent = serviceAccount.Name

	serviceAccountKey.Credentials = &iampb.ServiceAccountKey_PrivateKey{
		PrivateKey: &iampb.PrivateKey{},
	}

	grantIssuer := hydra.NewTrustOAuth2JwtGrantIssuer(
		// TODO: Determine how long we should allow a service account key to last if
		//       it's generated. Check what other providers do here.
		time.Now().AddDate(1, 0, 0),
		serviceAccount.ServiceAccountId,
		hydra.JsonWebKey{
			Alg: "RS256",
			Use: "sig",
			Kty: "RSA",
			Kid: serviceAccountKey.Uid,
			N:   hydra.PtrString(base64.URLEncoding.EncodeToString(publicKey.N.Bytes())),
			E:   hydra.PtrString(base64.URLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes())),
		},
		// TODO: Long-term we should investigate how we want to leverage the scope
		//       option to limit what access these tokens are provided.
		[]string{},
	)

	// Only allow the key to be used to generate JWT tokens for the service
	// account.
	grantIssuer.AllowAnySubject = hydra.PtrBool(false)
	grantIssuer.Subject = hydra.PtrString(serviceAccount.ServiceAccountId)

	_, _, err = s.hydraClient.TrustOAuth2JwtGrantIssuer(ctx).TrustOAuth2JwtGrantIssuer(*grantIssuer).Execute()
	if err != nil {
		return nil, err
	}

	_, err = s.keys.CreateResource(ctx, &storage.CreateResourceRequest[*iampb.ServiceAccountKey]{
		Parent:   req.Parent,
		Resource: serviceAccountKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to store resource: %s", err)
	}

	serviceAccountKey.GetPrivateKey().Key = base64.StdEncoding.EncodeToString(pem.EncodeToMemory(pemPrivateKey))

	return serviceAccountKey, nil
}

func (s *serviceAccounts) DeleteServiceAccountKey(ctx context.Context, req *iampb.DeleteServiceAccountKeyRequest) (*iampb.ServiceAccountKey, error) {
	key, err := s.keys.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Name,
	})
	if err != nil {
		return nil, err
	}

	serviceAccountEmail := key.Parent[strings.LastIndex(key.Parent, "/")+1:]

	grantIssuer, err := s.findGrantIssuer(ctx, serviceAccountEmail, key)
	if err != nil {
		return nil, err
	}

	_, err = s.hydraClient.DeleteTrustedOAuth2JwtGrantIssuer(ctx, *grantIssuer.Id).Execute()
	if err != nil {
		return nil, err
	}

	return s.keys.PurgeResource(ctx, &storage.PurgeResourceRequest{
		Name: req.Name,
	})
}

func (s *serviceAccounts) findGrantIssuer(ctx context.Context, serviceAccountEmail string, key *iampb.ServiceAccountKey) (*hydra.TrustedOAuth2JwtGrantIssuer, error) {
	resp, _, err := s.hydraClient.ListTrustedOAuth2JwtGrantIssuers(ctx).Issuer(serviceAccountEmail).MaxItems(100).Execute()
	if err != nil {
		return nil, err
	}

	for _, grantIssuer := range resp {
		if *grantIssuer.PublicKey.Kid == key.Uid {
			return &grantIssuer, nil
		}
	}

	return nil, fmt.Errorf("could not find a grant issuer for service account key '%s'", key.Name)
}

func createECDSAKeyPair() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate ECDSA key pair: %w", err)
	}
	return privateKey, &privateKey.PublicKey, nil
}
