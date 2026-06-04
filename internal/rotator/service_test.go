package rotator

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	cdnmocks "github.com/brunojet/go-infra-adapters/v3/pkg/cdn/mocks"
	cryptocontracts "github.com/brunojet/go-infra-adapters/v3/pkg/crypto/contracts"
	cryptomocks "github.com/brunojet/go-infra-adapters/v3/pkg/crypto/mocks"
	secretmocks "github.com/brunojet/go-infra-adapters/v3/pkg/secret/mocks"
	"github.com/golang/mock/gomock"

	"github.com/brunojet/go-edge-key-management/internal/domain"
)

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
	//nolint:gosec // Test fixture, not real credentials
	return RotationEvent{
		Step:               step,
		SecretId:           "arn:aws:secretsmanager:us-east-1:123:secret:test",
		ClientRequestToken: "test-token-0123456789012345678901",
	}
}

// --- getPendingIfValid ---

func TestGetPendingIfValid_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	payload := &domain.SecretPayload{
		PrivatePEM:   "private",
		PublicPEM:    "public",
		Fingerprint:  "fingerprint",
		CreatedAt:    time.Now().UTC(),
		KeyGroupName: "group",
		NamePrefix:   "prefix",
		PublicKeyID:  "key-id",
	}
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(payload, nil)
	svc := NewRotationService(store, cdnmocks.NewMockCdnAdapter(ctrl), testConfig(), discardLogger())

	got, err := svc.getPendingIfValid(context.Background(), testEvent("setSecret"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil payload")
	}
}

func TestGetPendingIfValid_GetVersionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(nil, errors.New("get failed"))
	svc := NewRotationService(store, cdnmocks.NewMockCdnAdapter(ctrl), testConfig(), discardLogger())

	_, err := svc.getPendingIfValid(context.Background(), testEvent("setSecret"))
	if err == nil {
		t.Error("expected error from GetVersion")
	}
}

func TestGetPendingIfValid_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(nil, nil)
	svc := NewRotationService(store, cdnmocks.NewMockCdnAdapter(ctrl), testConfig(), discardLogger())

	_, err := svc.getPendingIfValid(context.Background(), testEvent("setSecret"))
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %v", err)
	}
}

func TestGetPendingIfValid_Invalid(t *testing.T) {
	ctrl := gomock.NewController(t)
	payload := &domain.SecretPayload{} // Empty, invalid
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(payload, nil)
	store.EXPECT().DiscardVersion(gomock.Any(), gomock.Any()).Return(nil)
	cf := cdnmocks.NewMockCdnAdapter(ctrl)
	svc := NewRotationService(store, cf, testConfig(), discardLogger())

	_, err := svc.getPendingIfValid(context.Background(), testEvent("setSecret"))
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected 'invalid' error, got %v", err)
	}
}

func TestDiscardPending_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	payload := &domain.SecretPayload{PublicKeyID: "key-id-123"}
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().DiscardVersion(gomock.Any(), gomock.Any()).Return(nil)
	cf := cdnmocks.NewMockCdnAdapter(ctrl)
	cf.EXPECT().DeletePublicKey(gomock.Any(), "key-id-123").Return(nil)
	svc := NewRotationService(store, cf, testConfig(), discardLogger())

	svc.discardPending(context.Background(), testEvent("createSecret"), payload)
	// No error returned, just verifying calls were made (via gomock expectations)
}

func TestDiscardPending_NoPublicKeyID(t *testing.T) {
	ctrl := gomock.NewController(t)
	payload := &domain.SecretPayload{} // No PublicKeyID
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().DiscardVersion(gomock.Any(), gomock.Any()).Return(nil)
	cf := cdnmocks.NewMockCdnAdapter(ctrl)
	svc := NewRotationService(store, cf, testConfig(), discardLogger())

	svc.discardPending(context.Background(), testEvent("createSecret"), payload)
	// Should skip DeletePublicKey since PublicKeyID is empty
}

func TestDiscardPending_DeleteKeyError(t *testing.T) {
	ctrl := gomock.NewController(t)
	payload := &domain.SecretPayload{PublicKeyID: "key-id-123"}
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().DiscardVersion(gomock.Any(), gomock.Any()).Return(nil)
	cf := cdnmocks.NewMockCdnAdapter(ctrl)
	cf.EXPECT().DeletePublicKey(gomock.Any(), "key-id-123").Return(errors.New("delete failed"))
	svc := NewRotationService(store, cf, testConfig(), discardLogger())

	// Should not panic, just log error
	svc.discardPending(context.Background(), testEvent("createSecret"), payload)
}

func TestCleanupPending_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	payload := &domain.SecretPayload{PublicKeyID: "key-id-123"}
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(payload, nil)
	store.EXPECT().DiscardVersion(gomock.Any(), gomock.Any()).Return(nil)
	cf := cdnmocks.NewMockCdnAdapter(ctrl)
	cf.EXPECT().DeletePublicKey(gomock.Any(), "key-id-123").Return(nil)
	svc := NewRotationService(store, cf, testConfig(), discardLogger())

	svc.cleanupPending(context.Background(), testEvent("createSecret"))
}

func TestCleanupPending_GetVersionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(nil, errors.New("get failed"))
	svc := NewRotationService(store, cdnmocks.NewMockCdnAdapter(ctrl), testConfig(), discardLogger())

	// Should return early, not panic
	svc.cleanupPending(context.Background(), testEvent("createSecret"))
}

func TestCleanupPending_NilPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(nil, nil)
	svc := NewRotationService(store, cdnmocks.NewMockCdnAdapter(ctrl), testConfig(), discardLogger())

	// Should return early
	svc.cleanupPending(context.Background(), testEvent("createSecret"))
}

// --- Handle ---

func TestHandle_MissingToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	secretMock := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	cdnMock := cdnmocks.NewMockCdnAdapter(ctrl)
	svc := NewRotationService(secretMock, cdnMock, testConfig(), discardLogger())
	err := svc.Handle(context.Background(), RotationEvent{Step: "createSecret"})
	if err == nil {
		t.Fatal("expected error for missing ClientRequestToken")
	}
}

func TestHandle_UnknownStep(t *testing.T) {
	ctrl := gomock.NewController(t)
	secretMock := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	cdnMock := cdnmocks.NewMockCdnAdapter(ctrl)
	svc := NewRotationService(secretMock, cdnMock, testConfig(), discardLogger())
	err := svc.Handle(context.Background(), RotationEvent{Step: "unknownStep", ClientRequestToken: "tok"})
	if err == nil {
		t.Fatal("expected error for unknown step")
	}
}

// --- createSecret ---

func TestCreateSecret_PendingAlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	existingPayload := &domain.SecretPayload{
		PrivatePEM:   "valid-private",
		PublicPEM:    "valid-public",
		Fingerprint:  "valid-fingerprint",
		CreatedAt:    time.Now().UTC(),
		KeyGroupName: "test-group",
		NamePrefix:   "test",
	}
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(existingPayload, nil)
	store.EXPECT().DiscardVersion(gomock.Any(), gomock.Any()).Return(nil) // Discard existing pending
	store.EXPECT().GetCurrent(gomock.Any()).Return(&domain.SecretPayload{}, nil)
	cdnMock := cdnmocks.NewMockCdnAdapter(ctrl)
	cdnMock.EXPECT().CreatePublicKey(gomock.Any(), gomock.Any()).Return("key-id-123", nil)
	store.EXPECT().SetVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return("v-new", nil)
	svc := NewRotationService(store, cdnMock, testConfig(), discardLogger())
	if err := svc.Handle(context.Background(), testEvent("createSecret")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateSecret_PendingIncomplete(t *testing.T) {
	ctrl := gomock.NewController(t)
	// Incomplete payload with old CreatedAt (> minInterval ago) — gets discarded
	incompletePayload := &domain.SecretPayload{
		NamePrefix: "test",
		CreatedAt:  time.Now().Add(-2 * time.Hour), // 2 hours old, exceeds minInterval (60 min)
	}
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(incompletePayload, nil)
	store.EXPECT().DiscardVersion(gomock.Any(), gomock.Any()).Return(nil) // Traça 1: remove stale
	store.EXPECT().GetCurrent(gomock.Any()).Return(&domain.SecretPayload{}, nil)
	cdnMock := cdnmocks.NewMockCdnAdapter(ctrl)
	cdnMock.EXPECT().CreatePublicKey(gomock.Any(), gomock.Any()).Return("key-id-123", nil)
	store.EXPECT().SetVersion(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, payload *domain.SecretPayload, token string) (string, error) {
			return token, nil
		})
	svc := NewRotationService(store, cdnMock, testConfig(), discardLogger())
	if err := svc.Handle(context.Background(), testEvent("createSecret")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateSecret_RotationTooSoon(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(nil, nil)
	store.EXPECT().GetCurrent(gomock.Any()).Return(&domain.SecretPayload{CreatedAt: time.Now()}, nil)
	cfg := testConfig()
	cfg.MinRotationIntervalMinutes = 60
	svc := NewRotationService(store, cdnmocks.NewMockCdnAdapter(ctrl), cfg, discardLogger())
	err := svc.Handle(context.Background(), testEvent("createSecret"))
	if err == nil {
		t.Fatal("expected rotation-too-soon error")
	}
}

func TestCreateSecret_CurrentHasZeroCreatedAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(nil, nil)
	store.EXPECT().GetCurrent(gomock.Any()).Return(&domain.SecretPayload{CreatedAt: time.Time{}}, nil)
	cf := cdnmocks.NewMockCdnAdapter(ctrl)
	cf.EXPECT().CreatePublicKey(gomock.Any(), gomock.Any()).Return("key-id-123", nil)
	store.EXPECT().SetVersion(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, payload *domain.SecretPayload, token string) (string, error) {
			return token, nil
		})
	cfg := testConfig()
	cfg.MinRotationIntervalMinutes = 60
	svc := NewRotationService(store, cf, cfg, discardLogger())
	if err := svc.Handle(context.Background(), testEvent("createSecret")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateSecret_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(nil, nil)
	store.EXPECT().GetCurrent(gomock.Any()).Return(&domain.SecretPayload{}, nil)
	cf := cdnmocks.NewMockCdnAdapter(ctrl)
	cf.EXPECT().CreatePublicKey(gomock.Any(), gomock.Any()).Return("key-id-123", nil)
	store.EXPECT().SetVersion(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, payload *domain.SecretPayload, token string) (string, error) {
			return token, nil
		})
	svc := NewRotationService(store, cf, testConfig(), discardLogger())
	evt := testEvent("createSecret")
	if err := svc.Handle(context.Background(), evt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- setSecret ---

func TestSetSecret_PendingNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(nil, nil)
	svc := NewRotationService(store, cdnmocks.NewMockCdnAdapter(ctrl), testConfig(), discardLogger())
	err := svc.Handle(context.Background(), testEvent("setSecret"))
	if err == nil {
		t.Fatal("expected error when pending version not found")
	}
}

func TestSetSecret_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	pending := &domain.SecretPayload{
		PrivatePEM:   "private",
		PublicPEM:    "public",
		Fingerprint:  "abc12345def67890",
		CreatedAt:    time.Now().UTC(),
		KeyGroupName: "test-group",
		NamePrefix:   "test",
		PublicKeyID:  "key-id-123",
	}
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(pending, nil)
	cf := cdnmocks.NewMockCdnAdapter(ctrl)
	cf.EXPECT().EnsureKeyGroup(gomock.Any(), gomock.Any(), gomock.Any()).Return("group-id-456", nil)
	svc := NewRotationService(store, cf, testConfig(), discardLogger())
	if err := svc.Handle(context.Background(), testEvent("setSecret")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- testSecret ---

func TestTestSecret_KeyNotInGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	pending := &domain.SecretPayload{
		PrivatePEM:   "private",
		PublicPEM:    "public",
		Fingerprint:  "abc12345def67890",
		CreatedAt:    time.Now().UTC(),
		KeyGroupName: "test-group",
		NamePrefix:   "test",
		PublicKeyID:  "key-id-123",
	}
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(pending, nil)
	cf := cdnmocks.NewMockCdnAdapter(ctrl)
	cf.EXPECT().VerifyKeyInGroup(gomock.Any(), gomock.Any()).Return(false, nil)
	cf.EXPECT().DeletePublicKey(gomock.Any(), "key-id-123").Return(nil)
	svc := NewRotationService(store, cf, testConfig(), discardLogger())
	err := svc.Handle(context.Background(), testEvent("testSecret"))
	if err == nil {
		t.Fatal("expected error when key not in group")
	}
}

func TestTestSecret_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	pending := &domain.SecretPayload{
		PrivatePEM:   "private",
		PublicPEM:    "public",
		Fingerprint:  "abc12345def67890",
		CreatedAt:    time.Now().UTC(),
		KeyGroupName: "test-group",
		NamePrefix:   "test",
		PublicKeyID:  "key-id-123",
	}
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(pending, nil)
	cf := cdnmocks.NewMockCdnAdapter(ctrl)
	cf.EXPECT().VerifyKeyInGroup(gomock.Any(), gomock.Any()).Return(true, nil)
	svc := NewRotationService(store, cf, testConfig(), discardLogger())
	if err := svc.Handle(context.Background(), testEvent("testSecret")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- finishSecret ---

func TestFinishSecret_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().PromoteVersion(gomock.Any(), gomock.Any()).Return(nil)
	svc := NewRotationService(store, cdnmocks.NewMockCdnAdapter(ctrl), testConfig(), discardLogger())
	evt := testEvent("finishSecret")
	if err := svc.Handle(context.Background(), evt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFinishSecret_PromoteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().PromoteVersion(gomock.Any(), gomock.Any()).Return(errors.New("promote failed"))
	svc := NewRotationService(store, cdnmocks.NewMockCdnAdapter(ctrl), testConfig(), discardLogger())
	if err := svc.Handle(context.Background(), testEvent("finishSecret")); err == nil {
		t.Fatal("expected error from PromoteVersion")
	}
}

// --- createSecret errors ---

func TestCreateSecret_GetCurrentError(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(nil, nil)
	store.EXPECT().GetCurrent(gomock.Any()).Return(nil, errors.New("get current failed"))
	svc := NewRotationService(store, cdnmocks.NewMockCdnAdapter(ctrl), testConfig(), discardLogger())
	err := svc.Handle(context.Background(), testEvent("createSecret"))
	if err == nil {
		t.Fatal("expected error from GetCurrent")
	}
}

func TestCreateSecret_GenerateKeyError(t *testing.T) {
	ctrl := gomock.NewController(t)
	origNewRSAKeyGenerator := newRSAKeyGenerator
	defer func() { newRSAKeyGenerator = origNewRSAKeyGenerator }()

	mockKeyGen := cryptomocks.NewMockKeyGenerator(ctrl)
	mockKeyGen.EXPECT().Generate(gomock.Any()).Return(nil, errors.New("generate failed"))
	newRSAKeyGenerator = func(_ int) cryptocontracts.KeyGenerator {
		return mockKeyGen
	}

	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(nil, nil)
	store.EXPECT().GetCurrent(gomock.Any()).Return(&domain.SecretPayload{}, nil)
	svc := NewRotationService(store, cdnmocks.NewMockCdnAdapter(ctrl), testConfig(), discardLogger())
	err := svc.Handle(context.Background(), testEvent("createSecret"))
	if err == nil {
		t.Fatal("expected error from Generate")
	}
}

func TestCreateSecret_SetVersionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(nil, nil)
	store.EXPECT().GetCurrent(gomock.Any()).Return(&domain.SecretPayload{}, nil)
	cf := cdnmocks.NewMockCdnAdapter(ctrl)
	cf.EXPECT().CreatePublicKey(gomock.Any(), gomock.Any()).Return("key-id-123", nil)
	store.EXPECT().SetVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("set failed"))
	cf.EXPECT().DeletePublicKey(gomock.Any(), "key-id-123").Return(nil) // Sanitize on error
	svc := NewRotationService(store, cf, testConfig(), discardLogger())
	err := svc.Handle(context.Background(), testEvent("createSecret"))
	if err == nil {
		t.Fatal("expected error from SetVersion")
	}
}

func TestCreateSecret_CreatePublicKeyError(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(nil, nil)
	store.EXPECT().GetCurrent(gomock.Any()).Return(&domain.SecretPayload{}, nil)
	cf := cdnmocks.NewMockCdnAdapter(ctrl)
	cf.EXPECT().CreatePublicKey(gomock.Any(), gomock.Any()).Return("", errors.New("create key failed"))
	svc := NewRotationService(store, cf, testConfig(), discardLogger())
	err := svc.Handle(context.Background(), testEvent("createSecret"))
	if err == nil {
		t.Fatal("expected error from CreatePublicKey")
	}
}

// --- setSecret errors ---

func TestSetSecret_GetVersionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(nil, errors.New("get failed"))
	svc := NewRotationService(store, cdnmocks.NewMockCdnAdapter(ctrl), testConfig(), discardLogger())
	err := svc.Handle(context.Background(), testEvent("setSecret"))
	if err == nil {
		t.Fatal("expected error from GetVersion")
	}
}

func TestSetSecret_EnsureKeyGroupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	pending := &domain.SecretPayload{
		PrivatePEM:   "private",
		PublicPEM:    "public",
		Fingerprint:  "abc12345def67890",
		CreatedAt:    time.Now().UTC(),
		KeyGroupName: "test-group",
		NamePrefix:   "test",
		PublicKeyID:  "key-id-123",
	}
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(pending, nil)
	store.EXPECT().DiscardVersion(gomock.Any(), gomock.Any()).Return(nil) // Cleanup on error
	cf := cdnmocks.NewMockCdnAdapter(ctrl)
	cf.EXPECT().EnsureKeyGroup(gomock.Any(), gomock.Any(), "key-id-123").Return("", errors.New("ensure group failed"))
	cf.EXPECT().DeletePublicKey(gomock.Any(), "key-id-123").Return(nil) // Sanitize on error
	svc := NewRotationService(store, cf, testConfig(), discardLogger())
	err := svc.Handle(context.Background(), testEvent("setSecret"))
	if err == nil {
		t.Fatal("expected error from EnsureKeyGroup")
	}
}

// --- testSecret errors ---

func TestTestSecret_GetVersionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(nil, errors.New("get failed"))
	svc := NewRotationService(store, cdnmocks.NewMockCdnAdapter(ctrl), testConfig(), discardLogger())
	err := svc.Handle(context.Background(), testEvent("testSecret"))
	if err == nil {
		t.Fatal("expected error from GetVersion")
	}
}

func TestTestSecret_PendingNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(nil, nil)
	svc := NewRotationService(store, cdnmocks.NewMockCdnAdapter(ctrl), testConfig(), discardLogger())
	err := svc.Handle(context.Background(), testEvent("testSecret"))
	if err == nil {
		t.Fatal("expected error when pending not found")
	}
}

func TestTestSecret_VerifyKeyError(t *testing.T) {
	ctrl := gomock.NewController(t)
	pending := &domain.SecretPayload{
		PrivatePEM:   "private",
		PublicPEM:    "public",
		Fingerprint:  "abc12345def67890",
		CreatedAt:    time.Now().UTC(),
		KeyGroupName: "test-group",
		NamePrefix:   "test",
		PublicKeyID:  "key-id-123",
	}
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(pending, nil)
	cf := cdnmocks.NewMockCdnAdapter(ctrl)
	cf.EXPECT().VerifyKeyInGroup(gomock.Any(), gomock.Any()).Return(false, errors.New("verify failed"))
	cf.EXPECT().DeletePublicKey(gomock.Any(), "key-id-123").Return(nil)
	svc := NewRotationService(store, cf, testConfig(), discardLogger())
	err := svc.Handle(context.Background(), testEvent("testSecret"))
	if err == nil {
		t.Fatal("expected error from VerifyKeyInGroup")
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

// --- cdnKeyName ---

func TestCdnKeyName(t *testing.T) {
	tests := []struct {
		prefix      string
		fingerprint string
		want        string
	}{
		{"test", "abc12345def67890", "test-abc12345"},
		{"prod", "xyz", "prod-xyz"},
		{"short", "a", "short-a"},
		{"long", "0123456789abcdef", "long-01234567"},
	}
	for _, tt := range tests {
		got := cdnKeyName(tt.prefix, tt.fingerprint)
		if got != tt.want {
			t.Errorf("cdnKeyName(%q, %q) = %q, want %q", tt.prefix, tt.fingerprint, got, tt.want)
		}
	}
}
