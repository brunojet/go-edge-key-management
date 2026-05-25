package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/brunojet/go-edge-key-management/internal/rotator"
)

type mockSvc struct {
	handleFn func(ctx context.Context, event rotator.RotationEvent) error
}

func (m *mockSvc) Handle(ctx context.Context, event rotator.RotationEvent) error {
	return m.handleFn(ctx, event)
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
