package rotator

import (
	cdncontracts "github.com/brunojet/go-infra-adapters/pkg/cdn/contracts"
	secretcontracts "github.com/brunojet/go-infra-adapters/pkg/secret/contracts"
)

// SecretStore abstracts the secret backend (Secrets Manager, SSM, Vault, etc.).
// Re-exported from adapter for backward compatibility.
type SecretStore[T any] = secretcontracts.SecretStore[T]

// KeyDistribution abstracts the CDN key distribution layer (CloudFront, Fastly, etc.).
// Re-exported from adapter for backward compatibility.
type KeyDistribution = cdncontracts.KeyDistribution
