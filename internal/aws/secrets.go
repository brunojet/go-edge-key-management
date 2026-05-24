package awsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/brunojet/go-edge-key-management/internal/domain"
)

func GetPayload(ctx context.Context, sm SecretsManagerClient, secretName string) (*domain.SecretPayload, error) {
	out, err := sm.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		if IsNotFound(err) {
			if _, derr := sm.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
				SecretId: aws.String(secretName),
			}); derr != nil {
				if IsNotFound(derr) {
					return nil, fmt.Errorf("secret resource %q not found; create it via Terraform before invoking the rotator", secretName)
				}
				return nil, fmt.Errorf("describe secret: %w", derr)
			}
			log.Printf("Secret %s exists but has no value — will write initial payload", secretName)
			return &domain.SecretPayload{}, nil
		}
		return nil, fmt.Errorf("get secret value: %w", err)
	}
	if out.SecretString == nil {
		return &domain.SecretPayload{}, nil
	}
	var payload domain.SecretPayload
	if err := json.Unmarshal([]byte(*out.SecretString), &payload); err != nil {
		return nil, fmt.Errorf("unmarshal secret payload: %w", err)
	}
	log.Printf("Secret payload retrieved — version: %s", aws.ToString(out.VersionId))
	return &payload, nil
}

func PutPayload(ctx context.Context, sm SecretsManagerClient, secretName string, payload *domain.SecretPayload) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal secret payload: %w", err)
	}
	out, err := sm.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(secretName),
		SecretString: aws.String(string(b)),
	})
	if err != nil {
		return fmt.Errorf("put secret value: %w", err)
	}
	log.Printf("Secret saved — version: %s", aws.ToString(out.VersionId))
	return nil
}

func PutPayloadVersion(ctx context.Context, sm SecretsManagerClient, secretName string, payload *domain.SecretPayload, clientRequestToken string) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal secret payload: %w", err)
	}
	input := &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(secretName),
		SecretString: aws.String(string(b)),
	}
	if clientRequestToken != "" {
		input.ClientRequestToken = aws.String(clientRequestToken)
	}
	out, err := sm.PutSecretValue(ctx, input)
	if err != nil {
		return fmt.Errorf("put secret value (version): %w", err)
	}
	log.Printf("Secret pending version written — version: %s", aws.ToString(out.VersionId))
	prevPending, err := GetVersionWithStage(ctx, sm, secretName, "AWSPENDING")
	if err != nil {
		return fmt.Errorf("determine previous AWSPENDING: %w", err)
	}
	newVid := aws.ToString(out.VersionId)
	if prevPending == newVid {
		log.Printf("version %s already has AWSPENDING", newVid)
	} else {
		in := &secretsmanager.UpdateSecretVersionStageInput{
			SecretId:        aws.String(secretName),
			VersionStage:    aws.String("AWSPENDING"),
			MoveToVersionId: aws.String(newVid),
		}
		if prevPending != "" {
			in.RemoveFromVersionId = aws.String(prevPending)
		}
		if _, err := sm.UpdateSecretVersionStage(ctx, in); err != nil {
			return fmt.Errorf("update secret version stage to AWSPENDING: %w", err)
		}
	}
	time.Sleep(500 * time.Millisecond)
	return nil
}

func GetPayloadVersion(ctx context.Context, sm SecretsManagerClient, secretName, versionID string) (*domain.SecretPayload, error) {
	out, err := sm.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId:  aws.String(secretName),
		VersionId: aws.String(versionID),
	})
	if err != nil {
		return nil, fmt.Errorf("get secret value (version): %w", err)
	}
	if out.SecretString == nil {
		return &domain.SecretPayload{}, nil
	}
	var payload domain.SecretPayload
	if err := json.Unmarshal([]byte(*out.SecretString), &payload); err != nil {
		return nil, fmt.Errorf("unmarshal secret payload (version): %w", err)
	}
	return &payload, nil
}

func GetVersionWithStage(ctx context.Context, sm SecretsManagerClient, secretName, stage string) (string, error) {
	out, err := sm.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{SecretId: aws.String(secretName)})
	if err != nil {
		return "", fmt.Errorf("describe secret for versions: %w", err)
	}
	if out.VersionIdsToStages == nil {
		return "", nil
	}
	for vid, stages := range out.VersionIdsToStages {
		for _, s := range stages {
			if s == stage {
				return vid, nil
			}
		}
	}
	return "", nil
}

func PromoteVersionToCurrent(ctx context.Context, sm SecretsManagerClient, secretName, versionID, previousVersion string) error {
	in := &secretsmanager.UpdateSecretVersionStageInput{
		SecretId:        aws.String(secretName),
		VersionStage:    aws.String("AWSCURRENT"),
		MoveToVersionId: aws.String(versionID),
	}
	if previousVersion != "" {
		in.RemoveFromVersionId = aws.String(previousVersion)
	}
	if _, err := sm.UpdateSecretVersionStage(ctx, in); err != nil {
		return fmt.Errorf("promote version to AWSCURRENT: %w", err)
	}
	return nil
}

func RemoveVersionStage(ctx context.Context, sm SecretsManagerClient, secretName, stage, versionID string) error {
	in := &secretsmanager.UpdateSecretVersionStageInput{
		SecretId:            aws.String(secretName),
		VersionStage:        aws.String(stage),
		RemoveFromVersionId: aws.String(versionID),
	}
	if _, err := sm.UpdateSecretVersionStage(ctx, in); err != nil {
		return fmt.Errorf("remove version stage %s from %s: %w", stage, versionID, err)
	}
	return nil
}

func IsNotFound(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "resourcenotfoundexception") ||
		strings.Contains(msg, "not found")
}
