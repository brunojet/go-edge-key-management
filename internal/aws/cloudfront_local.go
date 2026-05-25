package awsclient

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cfTypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
)

// findPublicKeyIDByName searches the KeyGroup identified by groupName
// for a public key whose name matches keyName.
// Returns the key's CloudFront ID, or "" when not found.
func (s *CloudFrontService) findPublicKeyIDByName(ctx context.Context, groupName, keyName string) (string, error) {
	kg, err := s.getKeyGroupConfig(ctx, groupName)
	if err != nil {
		return "", err
	}
	if kg == nil {
		return "", nil
	}
	for _, keyID := range kg.Items {
		pkOut, err := s.client.GetPublicKey(ctx, &cloudfront.GetPublicKeyInput{Id: aws.String(keyID)})
		if err != nil {
			if IsNotFound(err) {
				continue
			}
			return "", fmt.Errorf("get public key %s: %w", keyID, err)
		}
		if pkOut.PublicKey != nil &&
			pkOut.PublicKey.PublicKeyConfig != nil &&
			aws.ToString(pkOut.PublicKey.PublicKeyConfig.Name) == keyName {
			return keyID, nil
		}
	}
	return "", nil
}

// getKeyGroupConfig resolves the KeyGroupConfig for the given group name.
// Returns nil (not an error) when the group does not exist.
func (s *CloudFrontService) getKeyGroupConfig(ctx context.Context, name string) (*cfTypes.KeyGroupConfig, error) {
	summary, err := s.findKeyGroupByName(ctx, name)
	if err != nil || summary == nil {
		return nil, err
	}
	got, err := s.client.GetKeyGroup(ctx, &cloudfront.GetKeyGroupInput{Id: summary.Id})
	if err != nil {
		return nil, fmt.Errorf("get key group %s: %w", aws.ToString(summary.Id), err)
	}
	if got.KeyGroup == nil || got.KeyGroup.KeyGroupConfig == nil {
		return nil, nil
	}
	return got.KeyGroup.KeyGroupConfig, nil
}

// findKeyGroupByName paginates ListKeyGroups until it finds a group matching name.
// Returns (nil, nil) when no group with that name exists — not an error.
func (s *CloudFrontService) findKeyGroupByName(ctx context.Context, name string) (*cfTypes.KeyGroup, error) {
	if name == "" {
		return nil, fmt.Errorf("keyGroupName must be provided")
	}
	var marker *string
	for {
		in := &cloudfront.ListKeyGroupsInput{MaxItems: aws.Int32(100)}
		if marker != nil {
			in.Marker = marker
		}
		out, err := s.client.ListKeyGroups(ctx, in)
		if err != nil {
			return nil, fmt.Errorf("list key groups: %w", err)
		}
		for _, item := range out.KeyGroupList.Items {
			if aws.ToString(item.KeyGroup.KeyGroupConfig.Name) == name {
				return &cfTypes.KeyGroup{
					Id:               item.KeyGroup.Id,
					LastModifiedTime: item.KeyGroup.LastModifiedTime,
					KeyGroupConfig:   item.KeyGroup.KeyGroupConfig,
				}, nil
			}
		}
		if aws.ToString(out.KeyGroupList.NextMarker) == "" {
			break
		}
		marker = out.KeyGroupList.NextMarker
	}
	return nil, nil
}

// createKeyGroup creates a new CloudFront KeyGroup with a single key.
func (s *CloudFrontService) createKeyGroup(ctx context.Context, name, publicKeyID string) (string, error) {
	out, err := s.client.CreateKeyGroup(ctx, &cloudfront.CreateKeyGroupInput{
		KeyGroupConfig: &cfTypes.KeyGroupConfig{
			Name:  aws.String(name),
			Items: []string{publicKeyID},
		},
	})
	if err != nil {
		return "", fmt.Errorf("create key group: %w", err)
	}
	id := aws.ToString(out.KeyGroup.Id)
	s.logger.Info("KeyGroup created", "id", id, "name", name)
	return id, nil
}

// updateKeyGroup refreshes the key list for an existing KeyGroup: prepends
// newPublicKeyID, fetches creation timestamps in parallel, keeps the newest
// s.maxKeys keys, and asynchronously deletes the evicted ones.
func (s *CloudFrontService) updateKeyGroup(ctx context.Context, keyGroupID, newPublicKeyID string) error {
	got, err := s.client.GetKeyGroup(ctx, &cloudfront.GetKeyGroupInput{Id: aws.String(keyGroupID)})
	if err != nil {
		return fmt.Errorf("get key group %s: %w", keyGroupID, err)
	}
	kg := got.KeyGroup.KeyGroupConfig
	candidates := deduplicateKeys(kg.Items, newPublicKeyID)

	metas, err := fetchKeyMetas(ctx, s.client, candidates, s.concurrency)
	if err != nil {
		return err
	}
	keep, evict := partitionByAge(metas, s.maxKeys)

	if _, err = s.client.UpdateKeyGroup(ctx, &cloudfront.UpdateKeyGroupInput{
		Id:             aws.String(keyGroupID),
		IfMatch:        got.ETag,
		KeyGroupConfig: &cfTypes.KeyGroupConfig{Name: kg.Name, Items: keep},
	}); err != nil {
		return fmt.Errorf("update key group %s: %w", keyGroupID, err)
	}
	s.logger.Info("KeyGroup updated", "id", keyGroupID, "active", keep, "evicted", evict)
	s.deleteOrphanKeys(ctx, evict)
	return nil
}

// deleteOrphanKeys attempts to delete each evicted key concurrently.
// Individual failures are logged but do not abort the remaining deletions.
func (s *CloudFrontService) deleteOrphanKeys(ctx context.Context, ids []string) {
	if len(ids) == 0 {
		return
	}
	sem := make(chan struct{}, s.concurrency)
	eg, ctx := errgroup.WithContext(ctx)
	for _, id := range ids {
		id := id
		sem <- struct{}{}
		eg.Go(func() error {
			defer func() { <-sem }()
			if err := s.deletePublicKeyWithETag(ctx, id); err != nil {
				s.logger.Error("failed deleting evicted key", "id", id, "error", err)
			}
			return nil
		})
	}
	_ = eg.Wait()
}

// deletePublicKeyWithETag fetches the current ETag for id and issues a conditional
// delete. Treats "not found" and "still in use" responses as non-errors.
func (s *CloudFrontService) deletePublicKeyWithETag(ctx context.Context, id string) error {
	got, err := s.client.GetPublicKey(ctx, &cloudfront.GetPublicKeyInput{Id: aws.String(id)})
	if err != nil {
		if IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get public key %s: %w", id, err)
	}
	_, derr := s.client.DeletePublicKey(ctx, &cloudfront.DeletePublicKeyInput{
		Id:      aws.String(id),
		IfMatch: got.ETag,
	})
	if derr == nil {
		s.logger.Info("deleted public key", "id", id)
		return nil
	}
	if IsNotFound(derr) {
		return nil
	}
	if IsStillInUse(derr) {
		s.logger.Info("cannot delete public key: still referenced", "id", id)
		return nil
	}
	return fmt.Errorf("delete public key %s: %w", id, derr)
}
