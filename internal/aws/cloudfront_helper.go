package awsclient

import (
	"context"
	"fmt"
	"sort"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
)

// keyMeta pairs a CloudFront public key ID with its creation timestamp.
type keyMeta struct {
	id        string
	createdAt time.Time
}

// deduplicateKeys returns a slice starting with newID followed by existing IDs,
// with duplicates removed. Relative order of existing is preserved.
func deduplicateKeys(existing []string, newID string) []string {
	seen := make(map[string]struct{}, len(existing)+1)
	result := make([]string, 0, len(existing)+1)
	result = append(result, newID)
	seen[newID] = struct{}{}
	for _, id := range existing {
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}
	return result
}

// partitionByAge sorts metas newest-first and splits at limit.
// Keys before limit go into keep; the rest into evict.
func partitionByAge(metas []keyMeta, limit int) (keep, evict []string) {
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].createdAt.After(metas[j].createdAt)
	})
	for i, m := range metas {
		if i < limit {
			keep = append(keep, m.id)
		} else {
			evict = append(evict, m.id)
		}
	}
	return keep, evict
}

// fetchKeyMetas retrieves creation timestamps for all ids concurrently,
// bounding parallelism to concurrency goroutines.
func fetchKeyMetas(ctx context.Context, cf CloudFrontClient, ids []string, concurrency int) ([]keyMeta, error) {
	if concurrency <= 0 {
		concurrency = 5
	}
	metas := make([]keyMeta, len(ids))
	sem := make(chan struct{}, concurrency)
	eg, ctx := errgroup.WithContext(ctx)

	for i, id := range ids {
		i, id := i, id
		sem <- struct{}{}
		eg.Go(func() error {
			defer func() { <-sem }()
			got, err := cf.GetPublicKey(ctx, &cloudfront.GetPublicKeyInput{Id: aws.String(id)})
			if err != nil {
				return fmt.Errorf("fetch metadata for key %s: %w", id, err)
			}
			km := keyMeta{id: id}
			if got.PublicKey != nil && got.PublicKey.CreatedTime != nil {
				km.createdAt = *got.PublicKey.CreatedTime
			}
			metas[i] = km
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return metas, nil
}
