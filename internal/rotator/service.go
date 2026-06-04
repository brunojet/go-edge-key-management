package rotator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/brunojet/go-infra-adapters/v3/pkg/crypto"

	"github.com/brunojet/go-edge-key-management/internal/domain"
)

// Injectable for testing.
var newRSAKeyGenerator = crypto.NewRSAKeyGenerator

// rsaKeyBits is the RSA key size used when generating new key pairs.
// CloudFront public keys support only 2048-bit RSA as of this writing.
const rsaKeyBits = 2048

// RotationEvent is the minimal subset of the Secrets Manager rotation event
// the Lambda receives.
type RotationEvent struct {
	Step               string `json:"Step"`
	SecretId           string `json:"SecretId"`
	ClientRequestToken string `json:"ClientRequestToken"`
}

// RotationService orchestrates the four-step Secrets Manager rotation contract.
// Each step is idempotent: re-invoking with the same ClientRequestToken is safe.
type RotationService struct {
	secrets    SecretStore[domain.SecretPayload]
	cloudfront KeyDistribution
	cfg        Config
	logger     *slog.Logger
}

// NewRotationService constructs a RotationService with the given dependencies.
func NewRotationService(secrets SecretStore[domain.SecretPayload], cf KeyDistribution, cfg Config, logger *slog.Logger) *RotationService {
	return &RotationService{
		secrets:    secrets,
		cloudfront: cf,
		cfg:        cfg,
		logger:     logger,
	}
}

// Handle dispatches event to the appropriate rotation step handler.
func (s *RotationService) Handle(ctx context.Context, event RotationEvent) error {
	if err := validateEvent(event); err != nil {
		return err
	}
	switch event.Step {
	case "createSecret":
		return s.createSecret(ctx, event)
	case "setSecret":
		return s.setSecret(ctx, event)
	case "testSecret":
		return s.testSecret(ctx, event)
	case "finishSecret":
		return s.finishSecret(ctx, event)
	default:
		return fmt.Errorf("rotation step %q not supported", event.Step)
	}
}

// createSecret generates a new RSA key pair, creates the public key in CloudFront,
// and stores it as the AWSPENDING version.
// No-op when a pending version already exists and is valid. Regenerates if incomplete.
// Enforces the minimum rotation interval.
// Detects stale/stuck AWSPENDING and aborts rapid retries to prevent loops.
func (s *RotationService) createSecret(ctx context.Context, event RotationEvent) error {
	s.cleanupPending(ctx, event)
	minInterval := time.Duration(s.cfg.MinRotationIntervalMinutes) * time.Minute
	if _, err := s.getCurrentWithIntervalCheck(ctx, minInterval); err != nil {
		return err
	}
	kp, err := newRSAKeyGenerator(rsaKeyBits).Generate(ctx)
	if err != nil {
		return fmt.Errorf("generate key pair: %w", err)
	}
	key := domain.CdnKey{
		Name:      cdnKeyName(s.cfg.NamePrefix, kp.Fingerprint),
		PEM:       string(kp.PublicPEM),
		GroupName: s.cfg.KeyGroupName,
	}
	pubID, err := s.cloudfront.CreatePublicKey(ctx, key)
	if err != nil {
		return fmt.Errorf("create public key: %w", err)
	}
	payload := &domain.SecretPayload{
		PrivatePEM:   string(kp.PrivatePEM),
		PublicPEM:    string(kp.PublicPEM),
		Fingerprint:  kp.Fingerprint,
		CreatedAt:    time.Now().UTC(),
		KeyGroupName: s.cfg.KeyGroupName,
		NamePrefix:   s.cfg.NamePrefix,
		PublicKeyID:  pubID,
	}
	if _, err := s.secrets.SetVersion(ctx, payload, event.ClientRequestToken); err != nil {
		if deleteErr := s.cloudfront.DeletePublicKey(ctx, pubID); deleteErr != nil {
			s.logger.Error("createSecret: failed to sanitize public key after secret store error", "keyID", pubID, "deleteErr", deleteErr)
		}
		return fmt.Errorf("store pending secret (public key sanitized): %w", err)
	}
	s.logger.Info("createSecret: public key created and pending version written", "keyID", pubID, "version", event.ClientRequestToken)
	return nil
}

// setSecret ensures the pending public key is added to the CloudFront KeyGroup.
// If KeyGroup update fails, sanitizes (removes) the public key before returning error.
func (s *RotationService) setSecret(ctx context.Context, event RotationEvent) error {
	pending, err := s.getPendingIfValid(ctx, event)
	if err != nil {
		return err
	}
	if _, err := s.cloudfront.EnsureKeyGroup(ctx, s.cfg.KeyGroupName, pending.PublicKeyID); err != nil {
		if deleteErr := s.cloudfront.DeletePublicKey(ctx, pending.PublicKeyID); deleteErr != nil {
			s.logger.Error("setSecret: failed to sanitize public key after keygroup error", "keyID", pending.PublicKeyID, "deleteErr", deleteErr)
		}
		_ = s.secrets.DiscardVersion(ctx, event.ClientRequestToken)
		return fmt.Errorf("ensure key group (public key sanitized): %w", err)
	}
	s.logger.Info("setSecret: public key added to key group", "keyID", pending.PublicKeyID, "keyGroup", s.cfg.KeyGroupName)
	return nil
}

// testSecret verifies that the pending public key is visible in the CloudFront KeyGroup.
func (s *RotationService) testSecret(ctx context.Context, event RotationEvent) error {
	pending, err := s.getPendingIfValid(ctx, event)
	if err != nil {
		return err
	}
	key := domain.CdnKey{
		Name:      cdnKeyName(s.cfg.NamePrefix, pending.Fingerprint),
		GroupName: s.cfg.KeyGroupName,
	}
	found, err := s.cloudfront.VerifyKeyInGroup(ctx, key)
	if err != nil {
		return fmt.Errorf("verify public key in key group: %w", err)
	}
	if !found {
		return fmt.Errorf("testSecret: pending public key not found in key group %s", s.cfg.KeyGroupName)
	}
	s.logger.Info("testSecret: pending key verified in key group", "version", event.ClientRequestToken, "keyGroup", s.cfg.KeyGroupName)
	return nil
}

// finishSecret promotes the pending version to AWSCURRENT.
// Idempotent: safe to retry with the same token.
func (s *RotationService) finishSecret(ctx context.Context, event RotationEvent) error {
	if err := s.secrets.PromoteVersion(ctx, event.ClientRequestToken); err != nil {
		return fmt.Errorf("finish secret: %w", err)
	}
	s.logger.Info("finishSecret: version promoted", "version", event.ClientRequestToken)
	return nil
}
