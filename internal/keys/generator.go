package keys

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/brunojet/go-edge-key-management/internal/domain"
)

// GenerateRSAKeyPair creates a new RSA key pair of the given bit size and
// returns PEM-encoded values along with a SHA-256 fingerprint of the public key.
func GenerateRSAKeyPair(bits int) (*domain.KeyPair, error) {
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, fmt.Errorf("rsa generate: %w", err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})

	fp := sha256.Sum256(pubBytes)

	return &domain.KeyPair{
		PrivatePEM:  string(privPEM),
		PublicPEM:   string(pubPEM),
		Fingerprint: hex.EncodeToString(fp[:]),
		CreatedAt:   time.Now().UTC(),
	}, nil
}
