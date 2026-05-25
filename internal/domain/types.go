// Package domain contains the core types shared across all packages.
// It has no dependencies on other internal packages, which prevents import cycles.
package domain

import "time"

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
}

// CdnKey holds only the data a CDN key distribution adapter needs.
// It contains no private key material.
type CdnKey struct {
	Name      string // key name in the CDN (e.g. "myapp-abc12345")
	PEM       string // PEM-encoded public key
	GroupName string // key group to associate with
}

