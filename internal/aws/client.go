package awsclient

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const cloudFrontRegion = "us-east-1"

type Clients struct {
	CloudFront     CloudFrontClient
	SecretsManager SecretsManagerClient
}

func NewClients(ctx context.Context) (*Clients, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	cfg.Retryer = NewRetryerProvider([]string{"InUse", "Conflict"}, 5)
	slog.Default().Info("AWS config loaded", "region", cfg.Region)
	if err := verifyIdentity(ctx, cfg); err != nil {
		return nil, fmt.Errorf("aws identity check: %w", err)
	}
	cfCfg := cfg.Copy()
	cfCfg.Region = cloudFrontRegion
	return &Clients{
		CloudFront:     cloudfront.NewFromConfig(cfCfg),
		SecretsManager: secretsmanager.NewFromConfig(cfg),
	}, nil
}

func verifyIdentity(ctx context.Context, cfg aws.Config) error {
	stsClient := sts.NewFromConfig(cfg)
	out, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("sts get-caller-identity: %w", err)
	}
	slog.Default().Info("AWS identity confirmed", "account", aws.ToString(out.Account), "arn", aws.ToString(out.Arn))
	return nil
}
