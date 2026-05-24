package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/brunojet/go-edge-key-management/internal/rotator"
)

func handler(ctx context.Context) error {
	secretName := os.Getenv("SECRET_NAME")
	if secretName == "" {
		secretName = "/go-edge-key-management/rotator"
	}

	keyGroupID := os.Getenv("KEY_GROUP_ID")
	namePrefix := os.Getenv("NAME_PREFIX")
	if namePrefix == "" {
		namePrefix = "go-edge"
	}

	log.Printf("Invoking rotation - secret=%s keyGroup=%s namePrefix=%s", secretName, keyGroupID, namePrefix)
	out, err := rotator.Rotate(ctx, secretName, keyGroupID, namePrefix)
	if err != nil {
		log.Printf("Rotation failed: %v", err)
		return err
	}

	if out != nil {
		log.Printf("Rotation succeeded - KeyGroupId=%s", out.KeyGroupId)
	} else {
		log.Printf("Rotation succeeded (no payload returned)")
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
