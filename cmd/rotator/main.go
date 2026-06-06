//go:build !debug
// +build !debug

// Package main implements the AWS Lambda handler for secret rotation.
package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/brunojet/go-edge-key-management/internal/handler"
)

// handlerNew is injectable for testing.
var handlerNew = handler.New

// lambdaStart is injectable for testing. Normally starts the Lambda handler.
var lambdaStart = lambda.Start

// logFatalf is injectable for testing (normally log.Fatalf).
var logFatalf = log.Fatalf

// startHandler initializes and starts the Lambda handler.
// Separated for testability.
func startHandler(ctx context.Context) error {
	h, err := handlerNew(ctx)
	if err != nil {
		return err
	}
	lambdaStart(h.Handle)
	return nil // unreachable unless lambdaStart returns (mocked)
}

// runMain is the main entry point, testable via dependency injection.
func runMain() error {
	if err := startHandler(context.Background()); err != nil {
		logFatalf("init: %v", err)
		return err
	}
	return nil
}

func main() {
	_ = runMain() // runMain calls logFatalf which exits, but we capture the error for testing
}
