package keys_test

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/brunojet/go-edge-key-management/internal/keys"
)

func TestGenerateRSAKeyPair(t *testing.T) {
	kp, err := keys.GenerateRSAKeyPair(2048)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("private key is valid PEM", func(t *testing.T) {
		block, _ := pem.Decode([]byte(kp.PrivatePEM))
		if block == nil {
			t.Fatal("PrivatePEM: failed to decode PEM block")
		}
		if block.Type != "RSA PRIVATE KEY" {
			t.Fatalf("PrivatePEM: got type %q, want %q", block.Type, "RSA PRIVATE KEY")
		}
		if _, err := x509.ParsePKCS1PrivateKey(block.Bytes); err != nil {
			t.Fatalf("PrivatePEM: parse failed: %v", err)
		}
	})

	t.Run("public key is valid PEM", func(t *testing.T) {
		block, _ := pem.Decode([]byte(kp.PublicPEM))
		if block == nil {
			t.Fatal("PublicPEM: failed to decode PEM block")
		}
		if block.Type != "PUBLIC KEY" {
			t.Fatalf("PublicPEM: got type %q, want %q", block.Type, "PUBLIC KEY")
		}
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			t.Fatalf("PublicPEM: parse failed: %v", err)
		}
		if _, ok := pub.(*rsa.PublicKey); !ok {
			t.Fatal("PublicPEM: expected *rsa.PublicKey")
		}
	})

	t.Run("fingerprint is 64-char hex", func(t *testing.T) {
		if len(kp.Fingerprint) != 64 {
			t.Fatalf("Fingerprint: got length %d, want 64", len(kp.Fingerprint))
		}
		if strings.ContainsAny(kp.Fingerprint, "ghijklmnopqrstuvwxyz") {
			t.Fatalf("Fingerprint: contains non-hex chars: %s", kp.Fingerprint)
		}
	})

	t.Run("created_at is set", func(t *testing.T) {
		if kp.CreatedAt.IsZero() {
			t.Fatal("CreatedAt must not be zero")
		}
	})

	t.Run("each call produces a unique key pair", func(t *testing.T) {
		kp2, err := keys.GenerateRSAKeyPair(2048)
		if err != nil {
			t.Fatalf("second generation failed: %v", err)
		}
		if kp.Fingerprint == kp2.Fingerprint {
			t.Fatal("two independently generated keys must have different fingerprints")
		}
	})
}
