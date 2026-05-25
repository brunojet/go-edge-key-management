package awsclient

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// movePendingStage assigns AWSPENDING to newVersionID, removing it from the
// previous holder when one exists. No-op when newVersionID already holds the stage.
func (s *SecretsService[T]) movePendingStage(ctx context.Context, secretName, newVersionID string) error {
	prevPending, err := s.getVersionWithStage(ctx, secretName, "AWSPENDING")
	if err != nil {
		return fmt.Errorf("determine previous AWSPENDING: %w", err)
	}
	if prevPending == newVersionID {
		s.logger.Info("version already has AWSPENDING", "version", newVersionID)
		return nil
	}
	in := &secretsmanager.UpdateSecretVersionStageInput{
		SecretId:        aws.String(secretName),
		VersionStage:    aws.String("AWSPENDING"),
		MoveToVersionId: aws.String(newVersionID),
	}
	if prevPending != "" {
		in.RemoveFromVersionId = aws.String(prevPending)
	}
	if _, err := s.client.UpdateSecretVersionStage(ctx, in); err != nil {
		return fmt.Errorf("update AWSPENDING: %w", err)
	}
	return nil
}

// getVersionWithStage returns the version ID currently holding stage, or ""
// when no version holds that stage.
func (s *SecretsService[T]) getVersionWithStage(ctx context.Context, secretName, stage string) (string, error) {
	out, err := s.client.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		return "", fmt.Errorf("describe secret: %w", err)
	}
	for vid, stages := range out.VersionIdsToStages {
		for _, stg := range stages {
			if stg == stage {
				return vid, nil
			}
		}
	}
	return "", nil
}
