package rotator

import (
	"context"
	"fmt"
	"time"

	"github.com/brunojet/go-edge-key-management/internal/domain"
)

// getPending retrieves the secret version identified by event.ClientRequestToken.
// Returns (nil, nil) when that version does not exist yet — not an error.
func (s *RotationService) getPending(ctx context.Context, event RotationEvent) (*domain.SecretPayload, error) {
	payload, err := s.secrets.GetVersion(ctx, event.ClientRequestToken)
	if err != nil {
		return nil, fmt.Errorf("get pending version: %w", err)
	}
	return payload, nil
}

func (s *RotationService) discardPending(ctx context.Context, event RotationEvent, payload *domain.SecretPayload) {
	if payload == nil {
		return
	}
	if payload.PublicKeyID != "" {
		if err := s.cloudfront.DeletePublicKey(ctx, payload.PublicKeyID); err != nil {
			s.logger.Error("cleanupPending: failed to delete public key", "keyID", payload.PublicKeyID, "error", err)
		}
	}
	if err := s.secrets.DiscardVersion(ctx, event.ClientRequestToken); err != nil {
		s.logger.Error("discardPending: failed to discard pending version", "version", event.ClientRequestToken, "error", err)
	} else {
		s.logger.Info("discardPending: pending version discarded", "version", event.ClientRequestToken)
	}
}

func (s *RotationService) cleanupPending(ctx context.Context, event RotationEvent) {
	payload, err := s.getPending(ctx, event)
	if err != nil {
		return
	}
	s.discardPending(ctx, event, payload)
}

func (s *RotationService) getPendingIfValid(ctx context.Context, event RotationEvent) (*domain.SecretPayload, error) {
	payload, err := s.getPending(ctx, event)
	if err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, fmt.Errorf("pending version %s not found", event.ClientRequestToken)
	}
	if !payload.IsValid() {
		s.discardPending(ctx, event, payload)
		return nil, fmt.Errorf("pending version %s is invalid and has been discarded", event.ClientRequestToken)
	}
	return payload, nil
}

// getCurrentWithIntervalCheck fetches the current secret and, when minInterval
// is positive, rejects the rotation if it is happening too soon after the last one.
func (s *RotationService) getCurrentWithIntervalCheck(ctx context.Context, minInterval time.Duration) (*domain.SecretPayload, error) {
	payload, err := s.secrets.GetCurrent(ctx)
	if err != nil {
		return nil, fmt.Errorf("get current payload: %w", err)
	}
	if minInterval > 0 && !payload.CreatedAt.IsZero() {
		if elapsed := time.Since(payload.CreatedAt); elapsed < minInterval {
			return nil, fmt.Errorf(
				"rotation attempted too soon: last rotation at %s, minimum interval %s",
				payload.CreatedAt.UTC().Format(time.RFC3339), minInterval,
			)
		}
	}
	return payload, nil
}
