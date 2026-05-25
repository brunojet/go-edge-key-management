package awsclient

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smTypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/brunojet/go-edge-key-management/internal/domain"
)

// mockSMClient implements SecretsManagerClient for tests.
type mockSMClient struct {
	getSecretValue           func(ctx context.Context, in *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	putSecretValue           func(ctx context.Context, in *secretsmanager.PutSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
	describeSecret           func(ctx context.Context, in *secretsmanager.DescribeSecretInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error)
	updateSecretVersionStage func(ctx context.Context, in *secretsmanager.UpdateSecretVersionStageInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretVersionStageOutput, error)
}

func (m *mockSMClient) GetSecretValue(ctx context.Context, in *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	return m.getSecretValue(ctx, in, opts...)
}
func (m *mockSMClient) PutSecretValue(ctx context.Context, in *secretsmanager.PutSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
	return m.putSecretValue(ctx, in, opts...)
}
func (m *mockSMClient) DescribeSecret(ctx context.Context, in *secretsmanager.DescribeSecretInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
	return m.describeSecret(ctx, in, opts...)
}
func (m *mockSMClient) UpdateSecretVersionStage(ctx context.Context, in *secretsmanager.UpdateSecretVersionStageInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretVersionStageOutput, error) {
	return m.updateSecretVersionStage(ctx, in, opts...)
}

func discardSlogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestSvc(client *mockSMClient) *SecretsService[domain.SecretPayload] {
	return NewSecretsService[domain.SecretPayload](client, discardSlogger())
}

// --- VerifyConnectivity ---

func TestVerifyConnectivity_OK(t *testing.T) {
	client := &mockSMClient{
		describeSecret: func(_ context.Context, _ *secretsmanager.DescribeSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
			return &secretsmanager.DescribeSecretOutput{ARN: aws.String("arn:aws:secretsmanager:us-east-1:123:secret:test")}, nil
		},
	}
	if err := newTestSvc(client).VerifyConnectivity(context.Background(), "test"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyConnectivity_SecretNotFound(t *testing.T) {
	client := &mockSMClient{
		describeSecret: func(_ context.Context, _ *secretsmanager.DescribeSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
			return nil, &smTypes.ResourceNotFoundException{Message: aws.String("secret not found")}
		},
	}
	err := newTestSvc(client).VerifyConnectivity(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
}

// --- GetCurrent ---

func TestGetCurrent_ReturnsPayload(t *testing.T) {
	payload := &domain.SecretPayload{NamePrefix: "test", Fingerprint: "abc"}
	b, _ := marshal[domain.SecretPayload](payload)
	client := &mockSMClient{
		getSecretValue: func(_ context.Context, _ *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			return &secretsmanager.GetSecretValueOutput{
				SecretString: aws.String(string(b)),
				VersionId:    aws.String("v1"),
			}, nil
		},
	}
	got, err := newTestSvc(client).GetCurrent(context.Background(), "my-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.NamePrefix != payload.NamePrefix {
		t.Errorf("NamePrefix = %q, want %q", got.NamePrefix, payload.NamePrefix)
	}
}

func TestGetCurrent_NoValue_ReturnsZero(t *testing.T) {
	// GetCurrent treats not-found as a new secret with no value yet (zero value).
	// Callers are expected to have verified connectivity via VerifyConnectivity first.
	client := &mockSMClient{
		getSecretValue: func(_ context.Context, _ *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			return nil, &smTypes.ResourceNotFoundException{Message: aws.String("secret has no value")}
		},
	}
	got, err := newTestSvc(client).GetCurrent(context.Background(), "new-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil zero payload")
	}
}

// --- GetPending ---

func TestGetPending_NotFound_ReturnsNil(t *testing.T) {
	client := &mockSMClient{
		getSecretValue: func(_ context.Context, _ *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			return nil, &smTypes.ResourceNotFoundException{Message: aws.String("version not found")}
		},
	}
	got, err := newTestSvc(client).GetPending(context.Background(), "secret", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestGetPending_Found_ReturnsPayload(t *testing.T) {
	payload := &domain.SecretPayload{NamePrefix: "pending-test"}
	b, _ := marshal[domain.SecretPayload](payload)
	client := &mockSMClient{
		getSecretValue: func(_ context.Context, _ *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			return &secretsmanager.GetSecretValueOutput{
				SecretString: aws.String(string(b)),
				VersionId:    aws.String("v2"),
			}, nil
		},
	}
	got, err := newTestSvc(client).GetPending(context.Background(), "secret", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.NamePrefix != payload.NamePrefix {
		t.Errorf("got %+v, want NamePrefix=%q", got, payload.NamePrefix)
	}
}

// --- PromotePending ---

func TestPromotePending_AlreadyCurrent_DiscardsOnly(t *testing.T) {
	const token = "my-token"
	discardCalled := false
	client := &mockSMClient{
		describeSecret: func(_ context.Context, _ *secretsmanager.DescribeSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
			return &secretsmanager.DescribeSecretOutput{
				VersionIdsToStages: map[string][]string{token: {"AWSCURRENT"}},
			}, nil
		},
		updateSecretVersionStage: func(_ context.Context, in *secretsmanager.UpdateSecretVersionStageInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretVersionStageOutput, error) {
			if aws.ToString(in.VersionStage) == "AWSPENDING" {
				discardCalled = true
			}
			return &secretsmanager.UpdateSecretVersionStageOutput{}, nil
		},
	}
	if err := newTestSvc(client).PromotePending(context.Background(), "secret", token); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !discardCalled {
		t.Error("expected DiscardPending to be called when token is already AWSCURRENT")
	}
}

func TestPromotePending_Promotes(t *testing.T) {
	const token = "new-version"
	const current = "old-version"
	promoteCalled := false
	client := &mockSMClient{
		describeSecret: func(_ context.Context, _ *secretsmanager.DescribeSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
			return &secretsmanager.DescribeSecretOutput{
				VersionIdsToStages: map[string][]string{current: {"AWSCURRENT"}},
			}, nil
		},
		updateSecretVersionStage: func(_ context.Context, in *secretsmanager.UpdateSecretVersionStageInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretVersionStageOutput, error) {
			if aws.ToString(in.VersionStage) == "AWSCURRENT" {
				promoteCalled = true
				if aws.ToString(in.MoveToVersionId) != token {
					t.Errorf("MoveToVersionId = %q, want %q", aws.ToString(in.MoveToVersionId), token)
				}
				if aws.ToString(in.RemoveFromVersionId) != current {
					t.Errorf("RemoveFromVersionId = %q, want %q", aws.ToString(in.RemoveFromVersionId), current)
				}
			}
			return &secretsmanager.UpdateSecretVersionStageOutput{}, nil
		},
	}
	if err := newTestSvc(client).PromotePending(context.Background(), "secret", token); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !promoteCalled {
		t.Error("expected AWSCURRENT stage update to be called")
	}
}
