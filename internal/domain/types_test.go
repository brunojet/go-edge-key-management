package domain

import (
	"testing"
	"time"
)

func TestKeyPair(t *testing.T) {
	now := time.Now().UTC()
	kp := KeyPair{
		PrivatePEM:  "private",
		PublicPEM:   "public",
		Fingerprint: "abc123",
		CreatedAt:   now,
	}

	if kp.PrivatePEM != "private" {
		t.Errorf("PrivatePEM: got %q, want %q", kp.PrivatePEM, "private")
	}
	if kp.PublicPEM != "public" {
		t.Errorf("PublicPEM: got %q, want %q", kp.PublicPEM, "public")
	}
	if kp.Fingerprint != "abc123" {
		t.Errorf("Fingerprint: got %q, want %q", kp.Fingerprint, "abc123")
	}
	if kp.CreatedAt != now {
		t.Errorf("CreatedAt: got %v, want %v", kp.CreatedAt, now)
	}
}

func TestSecretPayload(t *testing.T) {
	now := time.Now().UTC()
	sp := SecretPayload{
		PrivatePEM:   "private",
		PublicPEM:    "public",
		Fingerprint:  "abc123",
		CreatedAt:    now,
		KeyGroupName: "test-group",
		NamePrefix:   "test",
	}

	if sp.PrivatePEM != "private" {
		t.Errorf("PrivatePEM: got %q, want %q", sp.PrivatePEM, "private")
	}
	if sp.PublicPEM != "public" {
		t.Errorf("PublicPEM: got %q, want %q", sp.PublicPEM, "public")
	}
	if sp.Fingerprint != "abc123" {
		t.Errorf("Fingerprint: got %q, want %q", sp.Fingerprint, "abc123")
	}
	if sp.CreatedAt != now {
		t.Errorf("CreatedAt: got %v, want %v", sp.CreatedAt, now)
	}
	if sp.KeyGroupName != "test-group" {
		t.Errorf("KeyGroupName: got %q, want %q", sp.KeyGroupName, "test-group")
	}
	if sp.NamePrefix != "test" {
		t.Errorf("NamePrefix: got %q, want %q", sp.NamePrefix, "test")
	}
}

func TestSecretPayload_IsValid(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name    string
		payload *SecretPayload
		want    bool
	}{
		{
			name:    "nil payload",
			payload: nil,
			want:    false,
		},
		{
			name:    "empty payload",
			payload: &SecretPayload{},
			want:    false,
		},
		{
			name: "missing private pem",
			payload: &SecretPayload{
				PrivatePEM:   "",
				PublicPEM:    "public",
				Fingerprint:  "fingerprint",
				CreatedAt:    now,
				KeyGroupName: "group",
				NamePrefix:   "prefix",
			},
			want: false,
		},
		{
			name: "missing public pem",
			payload: &SecretPayload{
				PrivatePEM:   "private",
				PublicPEM:    "",
				Fingerprint:  "fingerprint",
				CreatedAt:    now,
				KeyGroupName: "group",
				NamePrefix:   "prefix",
			},
			want: false,
		},
		{
			name: "missing fingerprint",
			payload: &SecretPayload{
				PrivatePEM:   "private",
				PublicPEM:    "public",
				Fingerprint:  "",
				CreatedAt:    now,
				KeyGroupName: "group",
				NamePrefix:   "prefix",
			},
			want: false,
		},
		{
			name: "missing created at",
			payload: &SecretPayload{
				PrivatePEM:   "private",
				PublicPEM:    "public",
				Fingerprint:  "fingerprint",
				CreatedAt:    time.Time{},
				KeyGroupName: "group",
				NamePrefix:   "prefix",
			},
			want: false,
		},
		{
			name: "missing key group name",
			payload: &SecretPayload{
				PrivatePEM:   "private",
				PublicPEM:    "public",
				Fingerprint:  "fingerprint",
				CreatedAt:    now,
				KeyGroupName: "",
				NamePrefix:   "prefix",
			},
			want: false,
		},
		{
			name: "missing name prefix",
			payload: &SecretPayload{
				PrivatePEM:   "private",
				PublicPEM:    "public",
				Fingerprint:  "fingerprint",
				CreatedAt:    now,
				KeyGroupName: "group",
				NamePrefix:   "",
			},
			want: false,
		},
		{
			name: "complete payload",
			payload: &SecretPayload{
				PrivatePEM:   "private",
				PublicPEM:    "public",
				Fingerprint:  "fingerprint",
				CreatedAt:    now,
				KeyGroupName: "group",
				NamePrefix:   "prefix",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.payload.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
