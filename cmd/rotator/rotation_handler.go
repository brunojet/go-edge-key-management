package main

import (
	"context"
	"fmt"
	"log"
	"time"

	awsclient "github.com/brunojet/go-edge-key-management/internal/aws"
	"github.com/brunojet/go-edge-key-management/internal/domain"
	"github.com/brunojet/go-edge-key-management/internal/keys"
	"github.com/brunojet/go-edge-key-management/internal/rotator"
)

// rotationEvent is the minimal subset of the Secrets Manager rotation event
// the Lambda receives. We only care about Step, SecretId and ClientRequestToken.
type rotationEvent struct {
	Step               string `json:"Step"`
	SecretId           string `json:"SecretId"`
	ClientRequestToken string `json:"ClientRequestToken"`
}

// RotationHandler implements the Secrets Manager rotation contract steps.
func RotationHandler(ctx context.Context, event rotationEvent) error {
	cfg, err := rotator.Load()
	if err != nil {
		return err
	}
	clients, err := awsclient.NewClients(ctx)
	if err != nil {
		return err
	}
	if err := validateStep(event); err != nil {
		return err
	}
	switch event.Step {
	case "createSecret":
		pendingPayload, err := getPending(ctx, clients, event)
		if err != nil {
			return err
		}
		if pendingPayload != nil {
			log.Printf("pending version %s already present", event.ClientRequestToken)
			return nil
		}
		if _, err := getExistingWithIntervalCheck(ctx, clients, event, time.Duration(cfg.MinRotationIntervalMinutes)*time.Minute); err != nil {
			return err
		}
		kp, err := keys.GenerateRSAKeyPair(2048)
		if err != nil {
			return fmt.Errorf("generate key pair: %w", err)
		}
		payload := &domain.SecretPayload{
			PrivatePEM:   kp.PrivatePEM,
			PublicPEM:    kp.PublicPEM,
			Fingerprint:  kp.Fingerprint,
			CreatedAt:    kp.CreatedAt,
			KeyGroupName: cfg.KeyGroupName,
			NamePrefix:   cfg.NamePrefix,
		}
		if err := awsclient.PutPayloadVersion(ctx, clients.SecretsManager, event.SecretId, payload, event.ClientRequestToken); err != nil {
			return fmt.Errorf("put pending secret version: %w", err)
		}
		log.Printf("created pending secret version %s", event.ClientRequestToken)
		return nil

	case "setSecret":
		pendingPayload, err := getPending(ctx, clients, event)
		if err != nil {
			return err
		}
		if pendingPayload == nil {
			return fmt.Errorf("no pending version %s", event.ClientRequestToken)
		}
		pubID, err := awsclient.CreatePublicKey(ctx, clients.CloudFront, pendingPayload)
		if err != nil {
			return fmt.Errorf("create public key: %w", err)
		}
		if _, err := ensureKG(ctx, clients, cfg, pubID); err != nil {
			return fmt.Errorf("ensure key group in setSecret: %w", err)
		}
		log.Printf("setSecret: created public key %s and ensured key group %s", pubID, cfg.KeyGroupName)
		return nil

	case "testSecret":
		pending, err := getPending(ctx, clients, event)
		if err != nil {
			return err
		}
		if pending == nil {
			return fmt.Errorf("no pending version %s", event.ClientRequestToken)
		}
		foundID, err := awsclient.FindPublicKeyIDInKeyGroupByPEM(ctx, clients.CloudFront, cfg.KeyGroupName, pending.PublicPEM)
		if err != nil {
			return fmt.Errorf("search public key in key group: %w", err)
		}
		found := foundID != ""
		if !found {
			return fmt.Errorf("pending public key not found in key group %s", cfg.KeyGroupName)
		}
		log.Printf("testSecret succeeded for version %s", event.ClientRequestToken)
		return nil

	case "finishSecret":
		prev, err := awsclient.GetVersionWithStage(ctx, clients.SecretsManager, event.SecretId, "AWSCURRENT")
		if err != nil {
			return fmt.Errorf("find current version: %w", err)
		}
		if prev == event.ClientRequestToken {
			if err := awsclient.RemoveVersionStage(ctx, clients.SecretsManager, event.SecretId, "AWSPENDING", event.ClientRequestToken); err != nil {
				return fmt.Errorf("remove AWSPENDING from version %s: %w", event.ClientRequestToken, err)
			}
			log.Printf("removed AWSPENDING from version %s (already AWSCURRENT)", event.ClientRequestToken)
			return nil
		}
		if err := awsclient.PromoteVersionToCurrent(ctx, clients.SecretsManager, event.SecretId, event.ClientRequestToken, prev); err != nil {
			return fmt.Errorf("promote pending to current: %w", err)
		}
		log.Printf("version %s promoted to AWSCURRENT", event.ClientRequestToken)
		return nil

	default:
		return fmt.Errorf("rotation step %q not supported", event.Step)
	}
}
