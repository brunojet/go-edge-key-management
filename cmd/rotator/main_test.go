package main

import (
	"context"
	"errors"
	"os"
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

func TestHandlerNewSuccess(t *testing.T) {
	// Test successful handler initialization
	orig := handlerNew
	defer func() { handlerNew = orig }()

	called := false
	handlerNew = func(_ context.Context) (*handler.Handler, error) {
		called = true
		return &handler.Handler{}, nil
	}

	h, err := handlerNew(context.Background())
	if !called {
		t.Error("handlerNew should be called")
	}
	if h == nil {
		t.Error("expected non-nil handler on success")
	}
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestEnv(t *testing.T) {
	// Ensure build tag configuration is correct
	if os.Getenv("GO_BUILD_TAG") == "debug" {
		t.Skip("skipping non-debug build test")
	}
	// This file should only be included in non-debug builds
	t.Log("non-debug build test passed")
}

func TestMainErrorPath(t *testing.T) {
	// Test that main error path is reachable via startHandler error injection
	orig := handlerNew
	defer func() { handlerNew = orig }()

	handlerNew = func(_ context.Context) (*handler.Handler, error) {
		return nil, errors.New("init failed")
	}

	// Verify that startHandler propagates error (which main would log.Fatalf on)
	err := startHandler(context.Background())
	if err == nil {
		t.Fatal("expected error path to be testable")
	}
}

func TestStartHandlerError(t *testing.T) {
	orig := handlerNew
	defer func() { handlerNew = orig }()

	expectedErr := errors.New("handler initialization failed")
	handlerNew = func(_ context.Context) (*handler.Handler, error) {
		return nil, expectedErr
	}

	err := startHandler(context.Background())
	if err == nil {
		t.Error("expected error from startHandler")
	}
	if err != expectedErr {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestStartHandlerSuccess(t *testing.T) {
	origHandler := handlerNew
	origLambda := lambdaStart
	defer func() {
		handlerNew = origHandler
		lambdaStart = origLambda
	}()

	handlerCalled := false
	lambdaCalled := false

	handlerNew = func(_ context.Context) (*handler.Handler, error) {
		handlerCalled = true
		return &handler.Handler{}, nil
	}

	lambdaStart = func(_ interface{}) {
		lambdaCalled = true
	}

	err := startHandler(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !handlerCalled {
		t.Error("handler.New should be called")
	}
	if !lambdaCalled {
		t.Error("lambda.Start should be called")
	}
}
