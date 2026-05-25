package rotator

import (
	"context"

	"github.com/brunojet/go-edge-key-management/internal/domain"
)

// SecretStore abstracts the secret backend (Secrets Manager, SSM, Vault, etc.).
// T is the application-specific payload type stored as JSON.
// Implementations own the staging semantics (pending/current) — callers use
// domain-level operations only.
type SecretStore[T any] interface {
	GetCurrent(ctx context.Context, secretName string) (*T, error)
	SetPending(ctx context.Context, secretName string, payload *T, token string) error
	GetPending(ctx context.Context, secretName, token string) (*T, error)
	PromotePending(ctx context.Context, secretName, token string) error
	DiscardPending(ctx context.Context, secretName, token string) error
}

// KeyDistribution abstracts the CDN key distribution layer (CloudFront, Fastly, etc.).
// Implementations own the SDK translation — callers pass domain.CdnKey only.
type KeyDistribution interface {
	CreatePublicKey(ctx context.Context, key domain.CdnKey) (keyID string, err error)
	EnsureKeyGroup(ctx context.Context, name, keyID string) (groupID string, err error)
	VerifyKeyInGroup(ctx context.Context, key domain.CdnKey) (bool, error)
}
