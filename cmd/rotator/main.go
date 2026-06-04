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

// startHandler initializes and starts the Lambda handler.
// Separated for testability.
func startHandler(ctx context.Context) error {
	h, err := handlerNew(ctx)
	if err != nil {
		return err
	}
	lambda.Start(h.Handle)
	return nil // unreachable but satisfies return type
}

func main() {
	if err := startHandler(context.Background()); err != nil {
		log.Fatalf("init: %v", err)
	}
}
