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
	name             string
	getCurrent       func(ctx context.Context) (*domain.SecretPayload, error)
	getVersion       func(ctx context.Context, version string) (*domain.SecretPayload, error)
	setVersion       func(ctx context.Context, payload *domain.SecretPayload, version string) (string, error)
	promoteVersion   func(ctx context.Context, version string) error
	discardVersion   func(ctx context.Context, version string) error
}

func (m *mockSecretStore) Name() string {
	return m.name
}
func (m *mockSecretStore) GetCurrent(ctx context.Context) (*domain.SecretPayload, error) {
	return m.getCurrent(ctx)
}
func (m *mockSecretStore) GetVersion(ctx context.Context, version string) (*domain.SecretPayload, error) {
	return m.getVersion(ctx, version)
}
func (m *mockSecretStore) SetVersion(ctx context.Context, payload *domain.SecretPayload, version string) (string, error) {
	return m.setVersion(ctx, payload, version)
}
func (m *mockSecretStore) PromoteVersion(ctx context.Context, version string) error {
	return m.promoteVersion(ctx, version)
}
func (m *mockSecretStore) DiscardVersion(ctx context.Context, version string) error {
	return m.discardVersion(ctx, version)
}

type mockKeyDistribution struct {
	createPublicKey  func(ctx context.Context, key domain.CdnKey) (string, error)
	ensureKeyGroup   func(ctx context.Context, name, keyID string) (string, error)
	verifyKeyInGroup func(ctx context.Context, key domain.CdnKey) (bool, error)
	healthCheck      func(ctx context.Context) error
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
func (m *mockKeyDistribution) HealthCheck(ctx context.Context) error {
	if m.healthCheck == nil {
		return nil
	}
	return m.healthCheck(ctx)
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
		getVersion: func(_ context.Context, _ string) (*domain.SecretPayload, error) {
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
		getVersion: func(_ context.Context, _ string) (*domain.SecretPayload, error) {
			return nil, nil // no pending
		},
		getCurrent: func(_ context.Context) (*domain.SecretPayload, error) {
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
		getVersion: func(_ context.Context, _ string) (*domain.SecretPayload, error) {
			return nil, nil
		},
		getCurrent: func(_ context.Context) (*domain.SecretPayload, error) {
			return &domain.SecretPayload{}, nil
		},
		setVersion: func(_ context.Context, _ *domain.SecretPayload, token string) (string, error) {
			storedToken = token
			return token, nil
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
		getVersion: func(_ context.Context, _ string) (*domain.SecretPayload, error) {
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
		getVersion: func(_ context.Context, _ string) (*domain.SecretPayload, error) {
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
		getVersion: func(_ context.Context, _ string) (*domain.SecretPayload, error) {
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
		getVersion: func(_ context.Context, _ string) (*domain.SecretPayload, error) {
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
		promoteVersion: func(_ context.Context, token string) error {
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
		promoteVersion: func(_ context.Context, _ string) error {
			return errors.New("promote failed")
		},
	}
	svc := NewRotationService(store, &mockKeyDistribution{}, testConfig(), discardLogger())
	if err := svc.Handle(context.Background(), testEvent("finishSecret")); err == nil {
		t.Fatal("expected error from PromoteVersion")
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
