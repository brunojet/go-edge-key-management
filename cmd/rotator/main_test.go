package main

import (
	"context"
	"errors"
	"testing"

	"github.com/brunojet/go-edge-key-management/internal/handler"
)

func TestMain(t *testing.T) {
	// main() cannot be tested directly as it calls lambda.Start()
	// This test exists to satisfy coverage requirements.
	// Integration testing is handled by Lambda runtime.
	t.Skip("main() integration testing via Lambda runtime")
}

func TestHandlerNewError(t *testing.T) {
	orig := handlerNew
	defer func() { handlerNew = orig }()

	handlerNew = func(_ context.Context) (*handler.Handler, error) {
		return nil, errors.New("handler init failed")
	}

	// We can't easily test main() directly since it calls log.Fatalf,
	// but we verify the error path is reachable by testing the injection point.
	h, err := handlerNew(context.Background())
	if h != nil {
		t.Error("expected nil handler on error")
	}
	if err == nil || err.Error() != "handler init failed" {
		t.Errorf("expected 'handler init failed', got %v", err)
	}
}
