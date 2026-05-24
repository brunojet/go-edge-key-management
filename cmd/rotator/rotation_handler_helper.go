package main

import (
	"context"
	"fmt"
	"time"

	awsclient "github.com/brunojet/go-edge-key-management/internal/aws"
	"github.com/brunojet/go-edge-key-management/internal/domain"
	"github.com/brunojet/go-edge-key-management/internal/rotator"
)

func validateStep(event rotationEvent) error {
	if event.ClientRequestToken == "" {
		return fmt.Errorf("%s called without ClientRequestToken", event.Step)
	}
	return nil
}

func getPending(ctx context.Context, clients *awsclient.Clients, event rotationEvent) (*domain.SecretPayload, error) {
	payload, err := awsclient.GetPayloadVersion(ctx, clients.SecretsManager, event.SecretId, event.ClientRequestToken)
	if err != nil {
		if awsclient.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get pending version: %w", err)
	}
	if payload == nil {
		return nil, nil
	}
	return payload, nil
}

func getExisting(ctx context.Context, clients *awsclient.Clients, event rotationEvent) (*domain.SecretPayload, error) {
	payload, err := awsclient.GetPayload(ctx, clients.SecretsManager, event.SecretId)
	if err != nil {
		return nil, fmt.Errorf("get existing payload: %w", err)
	}
	return payload, nil
}

func getExistingWithIntervalCheck(ctx context.Context, clients *awsclient.Clients, event rotationEvent, minInterval time.Duration) (*domain.SecretPayload, error) {
	payload, err := getExisting(ctx, clients, event)
	if err != nil {
		return nil, err
	}
	if minInterval > 0 && !payload.CreatedAt.IsZero() {
		if time.Since(payload.CreatedAt) < minInterval {
			return nil, fmt.Errorf("rotation attempted too soon: last rotation at %s, minimum interval %s", payload.CreatedAt.UTC().Format(time.RFC3339), minInterval)
		}
	}
	return payload, nil
}

func ensureKG(ctx context.Context, clients *awsclient.Clients, cfg rotator.Config, pubID string) (string, error) {
	return awsclient.EnsureKeyGroup(ctx, clients.CloudFront, cfg.KeyGroupName, pubID)
}
