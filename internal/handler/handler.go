package handler

import (
	"context"
	"log/slog"

	cdnaws "github.com/brunojet/go-infra-adapters/pkg/cdn/aws"
	secretaws "github.com/brunojet/go-infra-adapters/pkg/secret/aws"

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

// New initialises adapters, verifies connectivity to both AWS services,
// builds the service graph, and returns a Handler.
func New(ctx context.Context) (*Handler, error) {
	cfg, err := rotator.Load()
	if err != nil {
		return nil, err
	}
	logger := slog.Default()

	// Initialize adapter services (adapters create AWS clients automatically)
	secretAPI, err := secretaws.NewSecretAPI(
		secretaws.WithLogger(logger),
	)
	if err != nil {
		return nil, err
	}
	smSvc := secretaws.NewSecrets[domain.SecretPayload](secretAPI, cfg.SecretName)
	// Verify secret connectivity by attempting to read current version
	if _, err := smSvc.GetCurrent(ctx); err != nil {
		return nil, err
	}

	cfSvc := cdnaws.NewCdn(
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
