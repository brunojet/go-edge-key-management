package awsclient

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cfTypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	"github.com/brunojet/go-edge-key-management/internal/domain"
)

// CloudFrontService implements rotator.KeyDistribution using the AWS CloudFront API.
type CloudFrontService struct {
	client      CloudFrontClient
	maxKeys     int
	concurrency int
	logger      *slog.Logger
}

// NewCloudFrontService constructs a CloudFrontService.
func NewCloudFrontService(client CloudFrontClient, maxKeys, concurrency int, logger *slog.Logger) *CloudFrontService {
	if maxKeys <= 0 {
		maxKeys = 3
	}
	if concurrency <= 0 {
		concurrency = 5
	}
	return &CloudFrontService{
		client:      client,
		maxKeys:     maxKeys,
		concurrency: concurrency,
		logger:      logger,
	}
}

// CreatePublicKey uploads the PEM-encoded public key described by key to CloudFront
// and returns the newly created key's ID.
func (s *CloudFrontService) CreatePublicKey(ctx context.Context, key domain.CdnKey) (string, error) {
	input := &cloudfront.CreatePublicKeyInput{
		PublicKeyConfig: &cfTypes.PublicKeyConfig{
			CallerReference: aws.String(fmt.Sprintf("%s-%d", key.Name, time.Now().UnixNano())),
			Name:            aws.String(key.Name),
			EncodedKey:      aws.String(key.PEM),
		},
	}
	out, err := s.client.CreatePublicKey(ctx, input)
	if err != nil {
		return "", fmt.Errorf("create public key: %w", err)
	}
	id := aws.ToString(out.PublicKey.Id)
	s.logger.Info("CloudFront public key created", "id", id, "name", key.Name)
	return id, nil
}

// EnsureKeyGroup guarantees a KeyGroup named name exists and contains keyID.
// Creates the group when absent, updates it otherwise.
// Returns the KeyGroup ID.
func (s *CloudFrontService) EnsureKeyGroup(ctx context.Context, name, keyID string) (string, error) {
	kg, err := s.findKeyGroupByName(ctx, name)
	if err != nil {
		return "", err
	}
	if kg == nil {
		s.logger.Info("KeyGroup not found, creating", "name", name)
		return s.createKeyGroup(ctx, name, keyID)
	}
	s.logger.Info("KeyGroup found", "id", aws.ToString(kg.Id), "name", name)
	if err := s.updateKeyGroup(ctx, aws.ToString(kg.Id), keyID); err != nil {
		return "", err
	}
	return aws.ToString(kg.Id), nil
}

// VerifyKeyInGroup reports whether the public key described by key exists
// in the KeyGroup identified by key.GroupName.
func (s *CloudFrontService) VerifyKeyInGroup(ctx context.Context, key domain.CdnKey) (bool, error) {
	id, err := s.findPublicKeyIDByName(ctx, key.GroupName, key.Name)
	return id != "", err
}

// VerifyConnectivity performs a lightweight ListPublicKeys call to confirm
// the CloudFront API is reachable with the current credentials.
func (s *CloudFrontService) VerifyConnectivity(ctx context.Context) error {
	if _, err := s.client.ListPublicKeys(ctx, &cloudfront.ListPublicKeysInput{MaxItems: aws.Int32(1)}); err != nil {
		return fmt.Errorf("cloudfront connectivity check: %w", err)
	}
	s.logger.Info("CloudFront connectivity confirmed")
	return nil
}
