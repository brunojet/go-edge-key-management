package rotator

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/brunojet/go-edge-key-management/internal/domain"
)

// --- mocks ---

type mockSecretStore struct {
	getCurrent     func(ctx context.Context, secretName string) (*domain.SecretPayload, error)
	setPending     func(ctx context.Context, secretName string, payload *domain.SecretPayload, token string) error
	getPending     func(ctx context.Context, secretName, token string) (*domain.SecretPayload, error)
	promotePending func(ctx context.Context, secretName, token string) error
	discardPending func(ctx context.Context, secretName, token string) error
}

func (m *mockSecretStore) GetCurrent(ctx context.Context, secretName string) (*domain.SecretPayload, error) {
	return m.getCurrent(ctx, secretName)
}
func (m *mockSecretStore) SetPending(ctx context.Context, secretName string, payload *domain.SecretPayload, token string) error {
	return m.setPending(ctx, secretName, payload, token)
}
func (m *mockSecretStore) GetPending(ctx context.Context, secretName, token string) (*domain.SecretPayload, error) {
	return m.getPending(ctx, secretName, token)
}
func (m *mockSecretStore) PromotePending(ctx context.Context, secretName, token string) error {
	return m.promotePending(ctx, secretName, token)
}
func (m *mockSecretStore) DiscardPending(ctx context.Context, secretName, token string) error {
	return m.discardPending(ctx, secretName, token)
}

type mockKeyDistribution struct {
	createPublicKey  func(ctx context.Context, key domain.CdnKey) (string, error)
	ensureKeyGroup   func(ctx context.Context, name, keyID string) (string, error)
	verifyKeyInGroup func(ctx context.Context, key domain.CdnKey) (bool, error)
}

func (m *mockKeyDistribution) CreatePublicKey(ctx context.Context, key domain.CdnKey) (string, error) {
	return m.createPublicKey(ctx, key)
}
func (m *mockKeyDistribution) EnsureKeyGroup(ctx context.Context, name, keyID string) (string, error) {
	return m.ensureKeyGroup(ctx, name, keyID)
}
func (m *mockKeyDistribution) VerifyKeyInGroup(ctx context.Context, key domain.CdnKey) (bool, error) {
	return m.verifyKeyInGroup(ctx, key)
}

// --- helpers ---

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testConfig() Config {
	return Config{
		KeyGroupName:               "test-group",
		NamePrefix:                 "test",
		MinRotationIntervalMinutes: 0,
		MaxKeysInGroup:             3,
		CloudFrontConcurrency:      5,
	}
}

func testEvent(step string) RotationEvent {
	return RotationEvent{
		Step:               step,
		SecretId:           "arn:aws:secretsmanager:us-east-1:123:secret:test",
		ClientRequestToken: "test-token-0123456789012345678901",
	}
}

// --- Handle ---

func TestHandle_MissingToken(t *testing.T) {
	svc := NewRotationService(&mockSecretStore{}, &mockKeyDistribution{}, testConfig(), discardLogger())
	err := svc.Handle(context.Background(), RotationEvent{Step: "createSecret"})
	if err == nil {
		t.Fatal("expected error for missing ClientRequestToken")
	}
}

func TestHandle_UnknownStep(t *testing.T) {
	svc := NewRotationService(&mockSecretStore{}, &mockKeyDistribution{}, testConfig(), discardLogger())
	err := svc.Handle(context.Background(), RotationEvent{Step: "unknownStep", ClientRequestToken: "tok"})
	if err == nil {
		t.Fatal("expected error for unknown step")
	}
}

// --- createSecret ---

func TestCreateSecret_PendingAlreadyExists(t *testing.T) {
	existingPayload := &domain.SecretPayload{NamePrefix: "test"}
	store := &mockSecretStore{
		getPending: func(_ context.Context, _, _ string) (*domain.SecretPayload, error) {
			return existingPayload, nil
		},
	}
	svc := NewRotationService(store, &mockKeyDistribution{}, testConfig(), discardLogger())
	if err := svc.Handle(context.Background(), testEvent("createSecret")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateSecret_RotationTooSoon(t *testing.T) {
	store := &mockSecretStore{
		getPending: func(_ context.Context, _, _ string) (*domain.SecretPayload, error) {
			return nil, nil // no pending
		},
		getCurrent: func(_ context.Context, _ string) (*domain.SecretPayload, error) {
			return &domain.SecretPayload{CreatedAt: time.Now()}, nil
		},
	}
	cfg := testConfig()
	cfg.MinRotationIntervalMinutes = 60
	svc := NewRotationService(store, &mockKeyDistribution{}, cfg, discardLogger())
	err := svc.Handle(context.Background(), testEvent("createSecret"))
	if err == nil {
		t.Fatal("expected rotation-too-soon error")
	}
}

func TestCreateSecret_Success(t *testing.T) {
	var storedToken string
	store := &mockSecretStore{
		getPending: func(_ context.Context, _, _ string) (*domain.SecretPayload, error) {
			return nil, nil
		},
		getCurrent: func(_ context.Context, _ string) (*domain.SecretPayload, error) {
			return &domain.SecretPayload{}, nil
		},
		setPending: func(_ context.Context, _ string, _ *domain.SecretPayload, token string) error {
			storedToken = token
			return nil
		},
	}
	svc := NewRotationService(store, &mockKeyDistribution{}, testConfig(), discardLogger())
	evt := testEvent("createSecret")
	if err := svc.Handle(context.Background(), evt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storedToken != evt.ClientRequestToken {
		t.Errorf("got token %q, want %q", storedToken, evt.ClientRequestToken)
	}
}

// --- setSecret ---

func TestSetSecret_PendingNotFound(t *testing.T) {
	store := &mockSecretStore{
		getPending: func(_ context.Context, _, _ string) (*domain.SecretPayload, error) {
			return nil, nil
		},
	}
	svc := NewRotationService(store, &mockKeyDistribution{}, testConfig(), discardLogger())
	err := svc.Handle(context.Background(), testEvent("setSecret"))
	if err == nil {
		t.Fatal("expected error when pending version not found")
	}
}

func TestSetSecret_Success(t *testing.T) {
	pending := &domain.SecretPayload{NamePrefix: "test", Fingerprint: "abc12345def67890", KeyGroupName: "test-group"}
	store := &mockSecretStore{
		getPending: func(_ context.Context, _, _ string) (*domain.SecretPayload, error) {
			return pending, nil
		},
	}
	cf := &mockKeyDistribution{
		createPublicKey: func(_ context.Context, _ domain.CdnKey) (string, error) {
			return "key-id-123", nil
		},
		ensureKeyGroup: func(_ context.Context, _, _ string) (string, error) {
			return "group-id-456", nil
		},
	}
	svc := NewRotationService(store, cf, testConfig(), discardLogger())
	if err := svc.Handle(context.Background(), testEvent("setSecret")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- testSecret ---

func TestTestSecret_KeyNotInGroup(t *testing.T) {
	pending := &domain.SecretPayload{NamePrefix: "test", Fingerprint: "abc12345def67890", KeyGroupName: "test-group"}
	store := &mockSecretStore{
		getPending: func(_ context.Context, _, _ string) (*domain.SecretPayload, error) {
			return pending, nil
		},
	}
	cf := &mockKeyDistribution{
		verifyKeyInGroup: func(_ context.Context, _ domain.CdnKey) (bool, error) {
			return false, nil
		},
	}
	svc := NewRotationService(store, cf, testConfig(), discardLogger())
	err := svc.Handle(context.Background(), testEvent("testSecret"))
	if err == nil {
		t.Fatal("expected error when key not in group")
	}
}

func TestTestSecret_Success(t *testing.T) {
	pending := &domain.SecretPayload{NamePrefix: "test", Fingerprint: "abc12345def67890", KeyGroupName: "test-group"}
	store := &mockSecretStore{
		getPending: func(_ context.Context, _, _ string) (*domain.SecretPayload, error) {
			return pending, nil
		},
	}
	cf := &mockKeyDistribution{
		verifyKeyInGroup: func(_ context.Context, _ domain.CdnKey) (bool, error) {
			return true, nil
		},
	}
	svc := NewRotationService(store, cf, testConfig(), discardLogger())
	if err := svc.Handle(context.Background(), testEvent("testSecret")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- finishSecret ---

func TestFinishSecret_Success(t *testing.T) {
	var promotedToken string
	store := &mockSecretStore{
		promotePending: func(_ context.Context, _ string, token string) error {
			promotedToken = token
			return nil
		},
	}
	svc := NewRotationService(store, &mockKeyDistribution{}, testConfig(), discardLogger())
	evt := testEvent("finishSecret")
	if err := svc.Handle(context.Background(), evt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if promotedToken != evt.ClientRequestToken {
		t.Errorf("promoted token %q, want %q", promotedToken, evt.ClientRequestToken)
	}
}

func TestFinishSecret_PromoteError(t *testing.T) {
	store := &mockSecretStore{
		promotePending: func(_ context.Context, _, _ string) error {
			return errors.New("promote failed")
		},
	}
	svc := NewRotationService(store, &mockKeyDistribution{}, testConfig(), discardLogger())
	if err := svc.Handle(context.Background(), testEvent("finishSecret")); err == nil {
		t.Fatal("expected error from PromotePending")
	}
}

// --- validateEvent ---

func TestValidateEvent(t *testing.T) {
	if err := validateEvent(RotationEvent{Step: "createSecret", ClientRequestToken: "tok"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := validateEvent(RotationEvent{Step: "createSecret"}); err == nil {
		t.Error("expected error for missing token")
	}
}
