// Package domain contains the core types shared across all packages.
// It has no dependencies on other internal packages, which prevents import cycles.
package domain

import (
	"time"

	cdncontracts "github.com/brunojet/go-infra-adapters/v4/pkg/cdn/contracts"
)

// KeyPair holds a generated RSA key pair and its metadata.
type KeyPair struct {
	PrivatePEM  string    `json:"private_pem"`
	PublicPEM   string    `json:"public_pem"`
	Fingerprint string    `json:"public_fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
}

// SecretPayload is the full JSON document written to Secrets Manager.
type SecretPayload struct {
	PrivatePEM   string    `json:"private_pem"`
	PublicPEM    string    `json:"public_pem"`
	Fingerprint  string    `json:"fingerprint"`
	CreatedAt    time.Time `json:"created_at"`
	KeyGroupName string    `json:"key_group_name"`
	NamePrefix   string    `json:"name_prefix"`
	PublicKeyID  string    `json:"public_key_id"`
}

// CdnKey re-exports the adapter type for backward compatibility.
type CdnKey = cdncontracts.CdnKey

// IsValid checks if the SecretPayload contains all required fields.
// Returns false if any field is empty or zero-valued.
func (p *SecretPayload) IsValid() bool {
	return p != nil &&
		p.PrivatePEM != "" &&
		p.PublicPEM != "" &&
		p.Fingerprint != "" &&
		!p.CreatedAt.IsZero() &&
		p.KeyGroupName != "" &&
		p.NamePrefix != "" &&
		p.PublicKeyID != ""
}
