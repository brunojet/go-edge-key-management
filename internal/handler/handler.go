package handler

import (
	"context"
	"log/slog"

	cdnaws "github.com/brunojet/go-infra-adapters/v4/pkg/cdn/aws"
	secretaws "github.com/brunojet/go-infra-adapters/v4/pkg/secret/aws"

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

// Injectable for testing.
var (
	rotatorLoad  = rotator.Load
	newSecretAPI = secretaws.NewSecretAPI
	newSecrets   = secretaws.NewSecrets[domain.SecretPayload]
	newCdn       = cdnaws.NewCdn
)

// New initializes adapters, verifies connectivity to both AWS services,
// builds the service graph, and returns a Handler.
func New(ctx context.Context) (*Handler, error) {
	cfg, err := rotatorLoad()
	if err != nil {
		return nil, err
	}
	logger := slog.Default()
	// Initialize adapter services (adapters create AWS clients automatically)
	secretAPI, err := newSecretAPI(
		secretaws.WithLogger(logger),
	)
	if err != nil {
		return nil, err
	}
	smSvc := newSecrets(secretAPI, cfg.SecretName)
	// Verify secret exists and credentials are valid (lightweight DescribeSecret check)
	if err := smSvc.HealthCheck(ctx); err != nil {
		return nil, err
	}

	cfSvc := newCdn(
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
