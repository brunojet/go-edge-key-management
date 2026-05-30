//go:build !debug
// +build !debug

package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/brunojet/go-edge-key-management/internal/handler"
)

// handlerNew is injectable for testing.
var handlerNew = handler.New

func main() {
	h, err := handlerNew(context.Background())
	if err != nil {
		log.Fatalf("init: %v", err)
	}
	lambda.Start(h.Handle)
}
