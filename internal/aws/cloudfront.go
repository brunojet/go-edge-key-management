package awsclient

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cfTypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	"github.com/brunojet/go-edge-key-management/internal/domain"
)

const (
	maxKeysInGroup     = 3
	defaultConcurrency = 5
)

func CreatePublicKey(ctx context.Context, cf CloudFrontClient, domain *domain.SecretPayload) (string, error) {
	input := &cloudfront.CreatePublicKeyInput{
		PublicKeyConfig: &cfTypes.PublicKeyConfig{
			CallerReference: aws.String(fmt.Sprintf("%s-%d", domain.NamePrefix, time.Now().UnixNano())),
			Name:            aws.String(domain.PublicKeyName()),
			EncodedKey:      aws.String(domain.PublicPEM),
		},
	}
	// Rely on the SDK client's configured retryer for transient retries.
	out, err := cf.CreatePublicKey(ctx, input)
	if err != nil {
		return "", fmt.Errorf("create public key: %w", err)
	}
	id := aws.ToString(out.PublicKey.Id)
	log.Printf("CloudFront public key created — id: %s, name: %s", id, domain.PublicKeyName())
	return id, nil
}

func EnsureKeyGroup(ctx context.Context, cf CloudFrontClient, keyGroupName, newPublicKeyID string) (string, error) {
	kg, err := findKeyGroupByName(ctx, cf, keyGroupName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("no KeyGroup named %q — creating", keyGroupName)
			return createKeyGroup(ctx, cf, keyGroupName, newPublicKeyID)
		}
		return "", err
	}
	log.Printf("KeyGroup found — id: %s, name: %s", aws.ToString(kg.Id), keyGroupName)
	if err := updateKeyGroup(ctx, cf, aws.ToString(kg.Id), newPublicKeyID); err != nil {
		return "", err
	}
	return aws.ToString(kg.Id), nil
}

func VerifyConnectivity(ctx context.Context, cf CloudFrontClient) error {
	if _, err := cf.ListPublicKeys(ctx, &cloudfront.ListPublicKeysInput{MaxItems: aws.Int32(1)}); err != nil {
		return fmt.Errorf("cloudfront connectivity check: %w", err)
	}
	log.Printf("CloudFront connectivity confirmed")
	return nil
}

func getPublicKeyMetadata(ctx context.Context, cf CloudFrontClient, id string) (string, *time.Time, error) {
	got, err := cf.GetPublicKey(ctx, &cloudfront.GetPublicKeyInput{Id: aws.String(id)})
	if err != nil {
		return "", nil, fmt.Errorf("get public key %s: %w", id, err)
	}
	var name string
	if got.PublicKey.PublicKeyConfig != nil {
		name = aws.ToString(got.PublicKey.PublicKeyConfig.Name)
	}
	if got.PublicKey.CreatedTime != nil {
		t := *got.PublicKey.CreatedTime
		return name, &t, nil
	}
	return name, nil, nil
}

func matchKeyGroup(items []cfTypes.KeyGroupSummary, name string) *cfTypes.KeyGroup {
	for _, s := range items {
		if aws.ToString(s.KeyGroup.KeyGroupConfig.Name) == name {
			return &cfTypes.KeyGroup{
				Id:               s.KeyGroup.Id, // ✅ via s.KeyGroup
				LastModifiedTime: s.KeyGroup.LastModifiedTime,
				KeyGroupConfig:   s.KeyGroup.KeyGroupConfig, // ✅ via s.KeyGroup
			}
		}
	}
	return nil
}

func findKeyGroupByName(ctx context.Context, cf CloudFrontClient, name string) (*cfTypes.KeyGroup, error) {
	if name == "" {
		return nil, fmt.Errorf("keyGroupName must be provided")
	}
	var marker *string
	for {
		in := &cloudfront.ListKeyGroupsInput{MaxItems: aws.Int32(100)}
		if marker != nil {
			in.Marker = marker
		}
		out, err := cf.ListKeyGroups(ctx, in)
		if err != nil {
			return nil, fmt.Errorf("list key groups: %w", err)
		}
		if kg := matchKeyGroup(out.KeyGroupList.Items, name); kg != nil {
			return kg, nil
		}
		if aws.ToString(out.KeyGroupList.NextMarker) == "" {
			break
		}
		marker = out.KeyGroupList.NextMarker
	}
	return nil, fmt.Errorf("key group with name %q not found", name)
}

func getKeyGroupByName(ctx context.Context, cf CloudFrontClient, name string) (*cloudfront.GetKeyGroupOutput, error) {
	kg, err := findKeyGroupByName(ctx, cf, name)
	if err != nil {
		return nil, err
	}
	got, err := cf.GetKeyGroup(ctx, &cloudfront.GetKeyGroupInput{Id: kg.Id})
	if err != nil {
		return nil, fmt.Errorf("get key group for test: %w", err)
	}
	return got, nil
}

func FindPublicKeyIDInKeyGroupByName(ctx context.Context, cf CloudFrontClient, domain *domain.SecretPayload) (string, error) {
	got, err := getKeyGroupByName(ctx, cf, domain.KeyGroupName)
	if err != nil {
		return "", err
	}
	if got == nil || got.KeyGroup == nil || got.KeyGroup.KeyGroupConfig == nil {
		return "", nil
	}
	for _, item := range got.KeyGroup.KeyGroupConfig.Items {
		pkOut, err := cf.GetPublicKey(ctx, &cloudfront.GetPublicKeyInput{Id: aws.String(item)})
		if err != nil {
			if isNotFoundCloudFront(err) {
				continue
			}
			return "", fmt.Errorf("get public key %s: %w", item, err)
		}
		if pkOut != nil && pkOut.PublicKey != nil && pkOut.PublicKey.PublicKeyConfig != nil {
			if aws.ToString(pkOut.PublicKey.PublicKeyConfig.Name) == domain.PublicKeyName() {
				return item, nil
			}
		}
	}
	return "", nil
}

func FindPublicKeyIDInKeyGroupByPEM(ctx context.Context, cf CloudFrontClient, keyGroupName, publicPEM string) (string, error) {
	got, err := getKeyGroupByName(ctx, cf, keyGroupName)
	if err != nil {
		return "", err
	}
	if got == nil || got.KeyGroup == nil || got.KeyGroup.KeyGroupConfig == nil {
		return "", nil
	}
	for _, item := range got.KeyGroup.KeyGroupConfig.Items {
		pkOut, err := cf.GetPublicKey(ctx, &cloudfront.GetPublicKeyInput{Id: aws.String(item)})
		if err != nil {
			if isNotFoundCloudFront(err) {
				continue
			}
			return "", fmt.Errorf("get public key %s: %w", item, err)
		}
		if pkOut != nil && pkOut.PublicKey != nil && pkOut.PublicKey.PublicKeyConfig != nil {
			if strings.TrimSpace(aws.ToString(pkOut.PublicKey.PublicKeyConfig.EncodedKey)) == strings.TrimSpace(publicPEM) {
				return item, nil
			}
		}
	}
	return "", nil
}

func createKeyGroup(ctx context.Context, cf CloudFrontClient, name, publicKeyID string) (string, error) {
	out, err := cf.CreateKeyGroup(ctx, &cloudfront.CreateKeyGroupInput{
		KeyGroupConfig: &cfTypes.KeyGroupConfig{
			Name:  aws.String(name),
			Items: []string{publicKeyID},
		},
	})
	if err != nil {
		return "", fmt.Errorf("create key group: %w", err)
	}
	id := aws.ToString(out.KeyGroup.Id)
	log.Printf("KeyGroup created — id: %s, name: %s", id, name)
	return id, nil
}

func updateKeyGroup(ctx context.Context, cf CloudFrontClient, keyGroupID, newPublicKeyID string) error {
	got, err := cf.GetKeyGroup(ctx, &cloudfront.GetKeyGroupInput{Id: aws.String(keyGroupID)})
	if err != nil {
		return fmt.Errorf("get key group %s: %w", keyGroupID, err)
	}
	kg := got.KeyGroup.KeyGroupConfig
	candidates := deduplicateKeys(kg.Items, newPublicKeyID)
	// fetch metas in parallel with limited concurrency
	metas, err := fetchKeyMetas(ctx, cf, candidates, defaultConcurrency)
	if err != nil {
		return err
	}
	keep, evict := partitionByAge(metas, maxKeysInGroup)
	if _, err = cf.UpdateKeyGroup(ctx, &cloudfront.UpdateKeyGroupInput{
		Id:             aws.String(keyGroupID),
		IfMatch:        got.ETag,
		KeyGroupConfig: &cfTypes.KeyGroupConfig{Name: kg.Name, Items: keep},
	}); err != nil {
		return fmt.Errorf("update key group %s: %w", keyGroupID, err)
	}
	log.Printf("KeyGroup %s updated — active: %v | evicted: %v", keyGroupID, keep, evict)
	deleteOrphanKeys(ctx, cf, keyGroupID, evict)
	return nil
}

type keyMeta struct {
	id        string
	createdAt time.Time
}

func deduplicateKeys(existing []string, newID string) []string {
	seen := make(map[string]struct{}, len(existing)+1)
	res := make([]string, 0, len(existing)+1)
	res = append(res, newID)
	seen[newID] = struct{}{}
	for _, id := range existing {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			res = append(res, id)
		}
	}
	return res
}

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

func fetchKeyMetas(ctx context.Context, cf CloudFrontClient, ids []string, concurrency int) ([]keyMeta, error) {
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}
	metas := make([]keyMeta, len(ids))
	eg, ctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, concurrency)
	for i, id := range ids {
		i, id := i, id
		eg.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()
			_, t, err := getPublicKeyMetadata(ctx, cf, id)
			if err != nil {
				return fmt.Errorf("fetch metadata for key %s: %w", id, err)
			}
			km := keyMeta{id: id}
			if t != nil {
				km.createdAt = *t
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

func deleteOrphanKeys(ctx context.Context, cf CloudFrontClient, excludeKGID string, ids []string) {
	if len(ids) == 0 {
		return
	}
	eg, ctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, defaultConcurrency)
	for _, id := range ids {
		id := id
		sem <- struct{}{}
		eg.Go(func() error {
			defer func() { <-sem }()
			if err := deletePublicKeyWithETag(ctx, cf, id); err != nil {
				log.Printf("failed deleting evicted key %s: %v", id, err)
			}
			// do not return the error to eg; we want other deletions to continue
			return nil
		})
	}
	_ = eg.Wait()
}

func deletePublicKeyWithETag(ctx context.Context, cf CloudFrontClient, id string) error {
	got, err := cf.GetPublicKey(ctx, &cloudfront.GetPublicKeyInput{Id: aws.String(id)})
	if err != nil {
		if isNotFoundCloudFront(err) {
			return nil
		}
		return fmt.Errorf("get public key %s: %w", id, err)
	}
	_, derr := cf.DeletePublicKey(ctx, &cloudfront.DeletePublicKeyInput{Id: aws.String(id), IfMatch: got.ETag})
	if derr == nil {
		log.Printf("deleted public key %s", id)
		return nil
	}
	if isNotFoundCloudFront(derr) {
		return nil
	}
	msg := strings.ToLower(derr.Error())
	if strings.Contains(msg, "in use") || strings.Contains(msg, "conflict") || strings.Contains(msg, "referenced") {
		log.Printf("cannot delete public key %s: still referenced (cloudfront message: %s)", id, msg)
		return nil
	}
	if derr != nil {
		return fmt.Errorf("delete public key %s: %w", id, derr)
	}
	return nil
}

func isNotFoundCloudFront(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "nosuch") ||
		strings.Contains(msg, "no such") ||
		strings.Contains(msg, "not found")
}
