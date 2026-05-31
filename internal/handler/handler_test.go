package handler

import (
	"context"
	"errors"
	"testing"

	cdnaws "github.com/brunojet/go-infra-adapters/v3/pkg/cdn/aws"
	cdncontracts "github.com/brunojet/go-infra-adapters/v3/pkg/cdn/contracts"
	cdnmocks "github.com/brunojet/go-infra-adapters/v3/pkg/cdn/mocks"
	secretaws "github.com/brunojet/go-infra-adapters/v3/pkg/secret/aws"
	secretcontracts "github.com/brunojet/go-infra-adapters/v3/pkg/secret/contracts"
	secretmocks "github.com/brunojet/go-infra-adapters/v3/pkg/secret/mocks"
	"github.com/golang/mock/gomock"

	"github.com/brunojet/go-edge-key-management/internal/domain"
	"github.com/brunojet/go-edge-key-management/internal/rotator"
)

type mockSvc struct {
	handleFn func(ctx context.Context, event rotator.RotationEvent) error
}

func (m *mockSvc) Handle(ctx context.Context, event rotator.RotationEvent) error {
	return m.handleFn(ctx, event)
}

func TestNew_ConfigLoadError(t *testing.T) {
	orig := rotatorLoad
	defer func() { rotatorLoad = orig }()

	rotatorLoad = func() (rotator.Config, error) {
		return rotator.Config{}, errors.New("config load failed")
	}

	_, err := New(context.Background())
	if err == nil || err.Error() != "config load failed" {
		t.Errorf("expected 'config load failed', got %v", err)
	}
}

func TestNew_SecretAPIError(t *testing.T) {
	origLoad := rotatorLoad
	origSecretAPI := newSecretAPI
	defer func() {
		rotatorLoad = origLoad
		newSecretAPI = origSecretAPI
	}()

	rotatorLoad = func() (rotator.Config, error) {
		return rotator.Config{SecretName: "test", MaxKeysInGroup: 3}, nil
	}

	newSecretAPI = func(...secretaws.Option) (*secretaws.SecretAPI, error) {
		return nil, errors.New("secret api init failed")
	}

	_, err := New(context.Background())
	if err == nil || err.Error() != "secret api init failed" {
		t.Errorf("expected 'secret api init failed', got %v", err)
	}
}

func TestNew_SecretHealthCheckError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	origLoad := rotatorLoad
	origSecretAPI := newSecretAPI
	origSecrets := newSecrets
	origCdn := newCdn
	defer func() {
		rotatorLoad = origLoad
		newSecretAPI = origSecretAPI
		newSecrets = origSecrets
		newCdn = origCdn
	}()

	rotatorLoad = func() (rotator.Config, error) {
		return rotator.Config{SecretName: "test", MaxKeysInGroup: 3}, nil
	}

	newSecretAPI = func(...secretaws.Option) (*secretaws.SecretAPI, error) {
		return nil, nil
	}

	newSecrets = func(_ *secretaws.SecretAPI, _ string) secretcontracts.SecretAdapter[domain.SecretPayload] {
		mockSecret := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
		mockSecret.EXPECT().HealthCheck(gomock.Any()).Return(errors.New("secret health check failed"))
		return mockSecret
	}

	_, err := New(context.Background())
	if err == nil || err.Error() != "secret health check failed" {
		t.Errorf("expected 'secret health check failed', got %v", err)
	}
}

func TestNew_CdnHealthCheckError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	origLoad := rotatorLoad
	origSecretAPI := newSecretAPI
	origSecrets := newSecrets
	origCdn := newCdn
	defer func() {
		rotatorLoad = origLoad
		newSecretAPI = origSecretAPI
		newSecrets = origSecrets
		newCdn = origCdn
	}()

	rotatorLoad = func() (rotator.Config, error) {
		return rotator.Config{SecretName: "test", MaxKeysInGroup: 3}, nil
	}

	newSecretAPI = func(...secretaws.Option) (*secretaws.SecretAPI, error) {
		return nil, nil
	}

	newSecrets = func(_ *secretaws.SecretAPI, _ string) secretcontracts.SecretAdapter[domain.SecretPayload] {
		mockSecret := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
		mockSecret.EXPECT().HealthCheck(gomock.Any()).Return(nil)
		return mockSecret
	}

	newCdn = func(...cdnaws.Option) cdncontracts.CdnAdapter {
		mockCdn := cdnmocks.NewMockCdnAdapter(ctrl)
		mockCdn.EXPECT().HealthCheck(gomock.Any()).Return(errors.New("cdn health check failed"))
		return mockCdn
	}

	_, err := New(context.Background())
	if err == nil || err.Error() != "cdn health check failed" {
		t.Errorf("expected 'cdn health check failed', got %v", err)
	}
}

func TestHandle_DelegatesToService(t *testing.T) {
	called := false
	h := &Handler{svc: &mockSvc{
		handleFn: func(_ context.Context, _ rotator.RotationEvent) error {
			called = true
			return nil
		},
	}}
	if err := h.Handle(context.Background(), rotator.RotationEvent{Step: "createSecret", ClientRequestToken: "tok"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("service Handle was not called")
	}
}

func TestHandle_PropagatesError(t *testing.T) {
	want := errors.New("service error")
	h := &Handler{svc: &mockSvc{
		handleFn: func(_ context.Context, _ rotator.RotationEvent) error {
			return want
		},
	}}
	if err := h.Handle(context.Background(), rotator.RotationEvent{}); !errors.Is(err, want) {
		t.Errorf("got error %v, want %v", err, want)
	}
}

func TestNew_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	origLoad := rotatorLoad
	origSecretAPI := newSecretAPI
	origSecrets := newSecrets
	origCdn := newCdn
	defer func() {
		rotatorLoad = origLoad
		newSecretAPI = origSecretAPI
		newSecrets = origSecrets
		newCdn = origCdn
	}()

	rotatorLoad = func() (rotator.Config, error) {
		return rotator.Config{SecretName: "test", MaxKeysInGroup: 3}, nil
	}

	newSecretAPI = func(...secretaws.Option) (*secretaws.SecretAPI, error) {
		return nil, nil
	}

	newSecrets = func(_ *secretaws.SecretAPI, _ string) secretcontracts.SecretAdapter[domain.SecretPayload] {
		mockSecret := secretmocks.NewMockSecretAdapter[domain.SecretPayload](ctrl)
		mockSecret.EXPECT().HealthCheck(gomock.Any()).Return(nil)
		return mockSecret
	}

	newCdn = func(...cdnaws.Option) cdncontracts.CdnAdapter {
		mockCdn := cdnmocks.NewMockCdnAdapter(ctrl)
		mockCdn.EXPECT().HealthCheck(gomock.Any()).Return(nil)
		return mockCdn
	}

	h, err := New(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h == nil {
		t.Fatal("expected handler, got nil")
	}
	if h.svc == nil {
		t.Error("expected handler with service, got nil svc")
	}
}
