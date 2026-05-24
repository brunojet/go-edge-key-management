package awsclient

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// CloudFrontClient abstracts the CloudFront SDK operations used by the rotator.
// This interface makes it straightforward to inject mocks in tests.
type CloudFrontClient interface {
	ListPublicKeys(ctx context.Context, input *cloudfront.ListPublicKeysInput, opts ...func(*cloudfront.Options)) (*cloudfront.ListPublicKeysOutput, error)
	ListKeyGroups(ctx context.Context, input *cloudfront.ListKeyGroupsInput, opts ...func(*cloudfront.Options)) (*cloudfront.ListKeyGroupsOutput, error)
	CreatePublicKey(ctx context.Context, input *cloudfront.CreatePublicKeyInput, opts ...func(*cloudfront.Options)) (*cloudfront.CreatePublicKeyOutput, error)
	GetPublicKey(ctx context.Context, input *cloudfront.GetPublicKeyInput, opts ...func(*cloudfront.Options)) (*cloudfront.GetPublicKeyOutput, error)
	DeletePublicKey(ctx context.Context, input *cloudfront.DeletePublicKeyInput, opts ...func(*cloudfront.Options)) (*cloudfront.DeletePublicKeyOutput, error)
	GetKeyGroup(ctx context.Context, input *cloudfront.GetKeyGroupInput, opts ...func(*cloudfront.Options)) (*cloudfront.GetKeyGroupOutput, error)
	CreateKeyGroup(ctx context.Context, input *cloudfront.CreateKeyGroupInput, opts ...func(*cloudfront.Options)) (*cloudfront.CreateKeyGroupOutput, error)
	UpdateKeyGroup(ctx context.Context, input *cloudfront.UpdateKeyGroupInput, opts ...func(*cloudfront.Options)) (*cloudfront.UpdateKeyGroupOutput, error)
}

// SecretsManagerClient abstracts the Secrets Manager SDK operations used by the rotator.
type SecretsManagerClient interface {
	GetSecretValue(ctx context.Context, input *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	PutSecretValue(ctx context.Context, input *secretsmanager.PutSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
	DescribeSecret(ctx context.Context, input *secretsmanager.DescribeSecretInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error)
	UpdateSecretVersionStage(ctx context.Context, input *secretsmanager.UpdateSecretVersionStageInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretVersionStageOutput, error)
}
