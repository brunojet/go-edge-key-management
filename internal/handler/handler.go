package handler

import (
	"context"
	"log/slog"

	awsclient "github.com/brunojet/go-edge-key-management/internal/aws"
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
	clients, err := awsclient.NewClients(ctx)
	if err != nil {
		return nil, err
	}
	logger := slog.Default()

	smSvc := awsclient.NewSecretsService[domain.SecretPayload](clients.SecretsManager, logger)
	if err := smSvc.VerifyConnectivity(ctx, cfg.SecretName); err != nil {
		return nil, err
	}

	cfSvc := awsclient.NewCloudFrontService(clients.CloudFront, cfg.MaxKeysInGroup, cfg.CloudFrontConcurrency, logger)
	if err := cfSvc.VerifyConnectivity(ctx); err != nil {
		return nil, err
	}

	svc := rotator.NewRotationService(smSvc, cfSvc, cfg, logger)
	return &Handler{svc: svc}, nil
}

// Handle is the Lambda entry point. Signature matches lambda.Start expectations.
func (h *Handler) Handle(ctx context.Context, event rotator.RotationEvent) error {
	return h.svc.Handle(ctx, event)
}
