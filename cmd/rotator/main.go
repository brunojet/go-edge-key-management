//go:build !debug
// +build !debug

package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/brunojet/go-edge-key-management/internal/handler"
)

func main() {
	h, err := handler.New(context.Background())
	if err != nil {
		log.Fatalf("init: %v", err)
	}
	lambda.Start(h.Handle)
}
