package rotator

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"time"
)

// KeyPair represents an RSA key pair and metadata.
type KeyPair struct {
	PrivatePEM  string    `json:"private_pem"`
	PublicPEM   string    `json:"public_pem"`
	Fingerprint string    `json:"public_fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
}

// GenerateRSAKeyPair generates an RSA key pair and returns PEM encoded values.
func GenerateRSAKeyPair(bits int) (*KeyPair, error) {
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, fmt.Errorf("rsa generate: %w", err)
	}

	priv := x509.MarshalPKCS1PrivateKey(key)
	privPem := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: priv})

	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public: %w", err)
	}
	pubPem := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})

	// fingerprint: SHA256 of public key bytes
	fp := sha256.Sum256(pubBytes)

	return &KeyPair{
		PrivatePEM:  string(privPem),
		PublicPEM:   string(pubPem),
		Fingerprint: hex.EncodeToString(fp[:]),
		CreatedAt:   time.Now().UTC(),
	}, nil
}

// SaveSecretLocal persists a JSON structure with current/previous keys to a local path (for testing).
// This is a stand-in for writing to AWS Secrets Manager in the production implemention.
func SaveSecretLocal(path string, current *KeyPair, previous *KeyPair) error {
	payload := map[string]interface{}{
		"current": current,
	}
	if previous != nil {
		payload["previous"] = previous
	}

	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal secret json: %w", err)
	}

	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("write secret file: %w", err)
	}
	return nil
}
