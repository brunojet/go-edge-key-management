package rotator

import (
	cdncontracts "github.com/brunojet/go-infra-adapters/v3/pkg/cdn/contracts"
	secretcontracts "github.com/brunojet/go-infra-adapters/v3/pkg/secret/contracts"
)

// SecretStore abstracts the secret backend (Secrets Manager, SSM, Vault, etc.).
// Re-exported from adapter for backward compatibility.
type SecretStore[T any] = secretcontracts.SecretAdapter[T]

// KeyDistribution abstracts the CDN key distribution layer (CloudFront, Fastly, etc.).
// Re-exported from adapter for backward compatibility.
type KeyDistribution = cdncontracts.CdnAdapter
