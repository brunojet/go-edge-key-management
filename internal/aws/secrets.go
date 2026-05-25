package awsclient

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// SecretsService implements rotator.SecretStore[T] using AWS Secrets Manager.
// T is the application-specific payload type stored as JSON — no secrets-manager
// coupling leaks into the caller.
type SecretsService[T any] struct {
	client SecretsManagerClient
	logger *slog.Logger
}

// NewSecretsService constructs a SecretsService for payload type T.
func NewSecretsService[T any](client SecretsManagerClient, logger *slog.Logger) *SecretsService[T] {
	return &SecretsService[T]{client: client, logger: logger}
}

// VerifyConnectivity confirms the secret resource exists and the AWS credentials
// are valid. Should be called once at startup before any rotation steps.
// Not part of the SecretStore port — this is a deployment-time check.
func (s *SecretsService[T]) VerifyConnectivity(ctx context.Context, secretName string) error {
	_, err := s.client.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		if IsNotFound(err) {
			return fmt.Errorf("secret %q not found; create it via Terraform before invoking the rotator", secretName)
		}
		return fmt.Errorf("secrets manager connectivity check: %w", err)
	}
	s.logger.Info("Secrets Manager connectivity confirmed", "secret", secretName)
	return nil
}

// GetCurrent retrieves and deserialises the current secret value into T.
// Returns a zero-value T when the secret exists but has no value yet (new secret).
// The caller is responsible for verifying the secret resource exists via
// VerifyConnectivity before calling this method.
func (s *SecretsService[T]) GetCurrent(ctx context.Context, secretName string) (*T, error) {
	out, err := s.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		if IsNotFound(err) {
			s.logger.Info("secret has no value yet, returning zero value", "secret", secretName)
			var zero T
			return &zero, nil
		}
		return nil, fmt.Errorf("get secret value: %w", err)
	}
	if out.SecretString == nil {
		var zero T
		return &zero, nil
	}
	v, err := unmarshal[T](*out.SecretString)
	if err != nil {
		return nil, err
	}
	s.logger.Info("secret payload retrieved", "version", aws.ToString(out.VersionId))
	return v, nil
}

// SetPending serialises payload, writes it as a new version identified by token,
// and moves the AWSPENDING stage to that version.
func (s *SecretsService[T]) SetPending(ctx context.Context, secretName string, payload *T, token string) error {
	b, err := marshal[T](payload)
	if err != nil {
		return err
	}
	input := &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(secretName),
		SecretString: aws.String(string(b)),
	}
	if token != "" {
		input.ClientRequestToken = aws.String(token)
	}
	out, err := s.client.PutSecretValue(ctx, input)
	if err != nil {
		return fmt.Errorf("put secret value: %w", err)
	}
	s.logger.Info("secret pending version written", "version", aws.ToString(out.VersionId))
	return s.movePendingStage(ctx, secretName, aws.ToString(out.VersionId))
}

// GetPending retrieves the secret version identified by token, deserialised into T.
// Returns (nil, nil) when that version does not exist yet.
func (s *SecretsService[T]) GetPending(ctx context.Context, secretName, token string) (*T, error) {
	out, err := s.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId:  aws.String(secretName),
		VersionId: aws.String(token),
	})
	if err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get pending version %s: %w", token, err)
	}
	if out.SecretString == nil {
		var zero T
		return &zero, nil
	}
	return unmarshal[T](*out.SecretString)
}

// PromotePending moves the secret identified by token from AWSPENDING to AWSCURRENT.
// Idempotent: if token is already AWSCURRENT, only the AWSPENDING stage is removed.
func (s *SecretsService[T]) PromotePending(ctx context.Context, secretName, token string) error {
	currentVersion, err := s.getVersionWithStage(ctx, secretName, "AWSCURRENT")
	if err != nil {
		return fmt.Errorf("find current version: %w", err)
	}
	if currentVersion == token {
		s.logger.Info("version already AWSCURRENT, removing AWSPENDING", "version", token)
		return s.DiscardPending(ctx, secretName, token)
	}
	in := &secretsmanager.UpdateSecretVersionStageInput{
		SecretId:        aws.String(secretName),
		VersionStage:    aws.String("AWSCURRENT"),
		MoveToVersionId: aws.String(token),
	}
	if currentVersion != "" {
		in.RemoveFromVersionId = aws.String(currentVersion)
	}
	if _, err := s.client.UpdateSecretVersionStage(ctx, in); err != nil {
		return fmt.Errorf("promote version to AWSCURRENT: %w", err)
	}
	s.logger.Info("version promoted to AWSCURRENT", "version", token)
	return nil
}

// DiscardPending removes the AWSPENDING stage from the given version.
func (s *SecretsService[T]) DiscardPending(ctx context.Context, secretName, token string) error {
	in := &secretsmanager.UpdateSecretVersionStageInput{
		SecretId:            aws.String(secretName),
		VersionStage:        aws.String("AWSPENDING"),
		RemoveFromVersionId: aws.String(token),
	}
	if _, err := s.client.UpdateSecretVersionStage(ctx, in); err != nil {
		return fmt.Errorf("discard AWSPENDING from version %s: %w", token, err)
	}
	return nil
}
