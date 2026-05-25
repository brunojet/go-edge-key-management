package handler

import (
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	cdnaws "github.com/brunojet/go-infra-adapters/internal/cdn/aws"
	secretaws "github.com/brunojet/go-infra-adapters/internal/secret/aws"

	"github.com/brunojet/go-edge-key-management/internal/domain"
	"github.com/brunojet/go-edge-key-management/internal/rotator"
)

// rotationSvc is the unexported interface that Handler depends on.
// Allows injecting a mock in tests.
type rotationSvc interface {
	Handle(ctx context.Context, event rotator.RotationEvent) error
}

// Handler is the Lambda adapter: it wires AWS clients + config into a
// RotationService and exposes a Handle method suitable for lambda.Start.
type Handler struct {
	svc rotationSvc
}

// New initialises AWS clients, verifies connectivity to both AWS services,
// builds the service graph, and returns a Handler.
func New(ctx context.Context) (*Handler, error) {
	cfg, err := rotator.Load()
	if err != nil {
		return nil, err
	}
	logger := slog.Default()

	// Load AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	// Verify identity
	stsClient := sts.NewFromConfig(awsCfg)
	out, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, err
	}
	logger.Info("AWS identity confirmed", "account", aws.ToString(out.Account), "arn", aws.ToString(out.Arn))

	// Create CloudFront client (must be in us-east-1)
	cfCfg := awsCfg.Copy()
	cfCfg.Region = "us-east-1"
	cfClient := cloudfront.NewFromConfig(cfCfg)

	// Create Secrets Manager client
	smClient := secretsmanager.NewFromConfig(awsCfg)

	// Initialize adapter services
	smSvc := secretaws.NewSecretsService[domain.SecretPayload](
		smClient,
		secretaws.WithLogger[domain.SecretPayload](logger),
	)
	if err := smSvc.HealthCheck(ctx, cfg.SecretName); err != nil {
		return nil, err
	}

	cfSvc := cdnaws.NewCloudFrontDistribution(
		cfClient,
		cdnaws.WithMaxKeys(cfg.MaxKeysInGroup),
		cdnaws.WithConcurrency(cfg.CloudFrontConcurrency),
		cdnaws.WithLogger(logger),
	)
	if err := cfSvc.HealthCheck(ctx); err != nil {
		return nil, err
	}

	svc := rotator.NewRotationService(smSvc, cfSvc, cfg, logger)
	return &Handler{svc: svc}, nil
}

// Handle is the Lambda entry point. Signature matches lambda.Start expectations.
func (h *Handler) Handle(ctx context.Context, event rotator.RotationEvent) error {
	return h.svc.Handle(ctx, event)
}
