package rotator

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/brunojet/go-edge-key-management/internal/domain"
	cdnmocks "github.com/brunojet/go-infra-adapters/v3/pkg/cdn/mocks"
	secretmocks "github.com/brunojet/go-infra-adapters/v3/pkg/secret/mocks"
	"github.com/golang/mock/gomock"
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
	return RotationEvent{
		Step:               step,
		SecretId:           "arn:aws:secretsmanager:us-east-1:123:secret:test",
		ClientRequestToken: "test-token-0123456789012345678901",
	}
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
	existingPayload := &domain.SecretPayload{NamePrefix: "test"}
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(existingPayload, nil)
	cdnMock := cdnmocks.NewMockCdnAdapter(ctrl)
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

func TestCreateSecret_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(nil, nil)
	store.EXPECT().GetCurrent(gomock.Any()).Return(&domain.SecretPayload{}, nil)
	store.EXPECT().SetVersion(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, payload *domain.SecretPayload, token string) (string, error) {
			return token, nil
		})
	svc := NewRotationService(store, cdnmocks.NewMockCdnAdapter(ctrl), testConfig(), discardLogger())
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
	pending := &domain.SecretPayload{NamePrefix: "test", Fingerprint: "abc12345def67890", KeyGroupName: "test-group"}
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(pending, nil)
	cf := cdnmocks.NewMockCdnAdapter(ctrl)
	cf.EXPECT().CreatePublicKey(gomock.Any(), gomock.Any()).Return("key-id-123", nil)
	cf.EXPECT().EnsureKeyGroup(gomock.Any(), gomock.Any(), gomock.Any()).Return("group-id-456", nil)
	svc := NewRotationService(store, cf, testConfig(), discardLogger())
	if err := svc.Handle(context.Background(), testEvent("setSecret")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- testSecret ---

func TestTestSecret_KeyNotInGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	pending := &domain.SecretPayload{NamePrefix: "test", Fingerprint: "abc12345def67890", KeyGroupName: "test-group"}
	store := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
	store.EXPECT().GetVersion(gomock.Any(), gomock.Any()).Return(pending, nil)
	cf := cdnmocks.NewMockCdnAdapter(ctrl)
	cf.EXPECT().VerifyKeyInGroup(gomock.Any(), gomock.Any()).Return(false, nil)
	svc := NewRotationService(store, cf, testConfig(), discardLogger())
	err := svc.Handle(context.Background(), testEvent("testSecret"))
	if err == nil {
		t.Fatal("expected error when key not in group")
	}
}

func TestTestSecret_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	pending := &domain.SecretPayload{NamePrefix: "test", Fingerprint: "abc12345def67890", KeyGroupName: "test-group"}
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

// --- validateEvent ---

func TestValidateEvent(t *testing.T) {
	if err := validateEvent(RotationEvent{Step: "createSecret", ClientRequestToken: "tok"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := validateEvent(RotationEvent{Step: "createSecret"}); err == nil {
		t.Error("expected error for missing token")
	}
}
