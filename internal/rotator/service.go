package rotator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/brunojet/go-edge-key-management/internal/domain"
	"github.com/brunojet/go-infra-adapters/v3/pkg/crypto"
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

// createSecret generates a new RSA key pair and stores it as the AWSPENDING version.
// No-op when a pending version already exists and is valid. Regenerates if incomplete.
// Enforces the minimum rotation interval.
func (s *RotationService) createSecret(ctx context.Context, event RotationEvent) error {
	pending, err := s.getPending(ctx, event)
	if err != nil {
		return err
	}
	if pending != nil && pending.IsValid() {
		s.logger.Info("pending version already present — skipping createSecret", "version", event.ClientRequestToken)
		return nil
	}
	minInterval := time.Duration(s.cfg.MinRotationIntervalMinutes) * time.Minute
	if _, err := s.getCurrentWithIntervalCheck(ctx, minInterval); err != nil {
		return err
	}
	kp, err := newRSAKeyGenerator(rsaKeyBits).Generate(ctx)
	if err != nil {
		return fmt.Errorf("generate key pair: %w", err)
	}
	payload := &domain.SecretPayload{
		PrivatePEM:   string(kp.PrivatePEM),
		PublicPEM:    string(kp.PublicPEM),
		Fingerprint:  kp.Fingerprint,
		CreatedAt:    time.Now().UTC(),
		KeyGroupName: s.cfg.KeyGroupName,
		NamePrefix:   s.cfg.NamePrefix,
	}
	if _, err := s.secrets.SetVersion(ctx, payload, event.ClientRequestToken); err != nil {
		return fmt.Errorf("store pending secret: %w", err)
	}
	s.logger.Info("createSecret: pending version written", "version", event.ClientRequestToken)
	return nil
}

// setSecret uploads the pending public key to CloudFront and ensures the KeyGroup contains it.
func (s *RotationService) setSecret(ctx context.Context, event RotationEvent) error {
	pending, err := s.getPending(ctx, event)
	if err != nil {
		return err
	}
	if pending == nil {
		return fmt.Errorf("setSecret: pending version %s not found", event.ClientRequestToken)
	}
	key := domain.CdnKey{
		Name:      cdnKeyName(s.cfg.NamePrefix, pending.Fingerprint),
		PEM:       pending.PublicPEM,
		GroupName: s.cfg.KeyGroupName,
	}
	pubID, err := s.cloudfront.CreatePublicKey(ctx, key)
	if err != nil {
		return fmt.Errorf("create public key: %w", err)
	}
	if _, err := s.cloudfront.EnsureKeyGroup(ctx, s.cfg.KeyGroupName, pubID); err != nil {
		return fmt.Errorf("ensure key group: %w", err)
	}
	s.logger.Info("setSecret: public key added to key group", "keyID", pubID, "keyGroup", s.cfg.KeyGroupName)
	return nil
}

// testSecret verifies that the pending public key is visible in the CloudFront KeyGroup.
func (s *RotationService) testSecret(ctx context.Context, event RotationEvent) error {
	pending, err := s.getPending(ctx, event)
	if err != nil {
		return err
	}
	if pending == nil {
		return fmt.Errorf("testSecret: pending version %s not found", event.ClientRequestToken)
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
