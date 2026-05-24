package rotator

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultNamePrefix = "go-edge"
)

// Config holds every parameter the rotator needs to run.
// All fields are populated from environment variables via Load.
type Config struct {
	// KeyGroupName, when provided, is used as the KeyGroup's Name when
	// creating a new KeyGroup. `KeyGroupID` takes precedence over this value.
	KeyGroupName string

	// NamePrefix is the prefix used when naming CloudFront resources
	// (public keys, key groups).
	NamePrefix string

	// KeyRetentionDays controls automatic deletion of old public keys (in days).
	// When 0 the retention-based deletion is disabled.
	KeyRetentionDays int

	// MinPublicKeysToKeep ensures we never delete more than leaving this many
	// public keys in use. Defaults to 2 to preserve rotation safety.
	MinPublicKeysToKeep int

	// OnlyDeleteManagedKeys restricts deletion to keys the rotator created
	// (identified by NamePrefix). Defaults to true.
	OnlyDeleteManagedKeys bool

	// MinRotationIntervalMinutes prevents rotations that occur more frequently
	// than the configured interval (in minutes). When 0 the check is disabled.
	MinRotationIntervalMinutes int
}

// Load reads configuration from environment variables and applies defaults.
// Returns an error when a required variable is absent or invalid.
func Load() (Config, error) {
	cfg := Config{
		KeyGroupName:               os.Getenv("KEY_GROUP_NAME"),
		NamePrefix:                 envOrDefault("NAME_PREFIX", defaultNamePrefix),
		KeyRetentionDays:           envOrDefaultInt("KEY_RETENTION_DAYS", 30),
		MinPublicKeysToKeep:        envOrDefaultInt("MIN_PUBLIC_KEYS_TO_KEEP", 2),
		OnlyDeleteManagedKeys:      envOrDefaultBool("ONLY_DELETE_MANAGED_KEYS", true),
		MinRotationIntervalMinutes: envOrDefaultInt("MIN_ROTATION_INTERVAL_MINUTES", 60),
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) validate() error {
	if c.KeyGroupName == "" {
		return fmt.Errorf("KEY_GROUP_NAME must be set in environment")
	}
	if c.MinPublicKeysToKeep < 1 {
		return fmt.Errorf("MIN_PUBLIC_KEYS_TO_KEEP must be >= 1")
	}
	if c.KeyRetentionDays < 0 {
		return fmt.Errorf("KEY_RETENTION_DAYS must be >= 0")
	}
	if c.MinRotationIntervalMinutes < 0 {
		return fmt.Errorf("MIN_ROTATION_INTERVAL_MINUTES must be >= 0")
	}
	return nil
}

func envOrDefaultInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func envOrDefaultBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
	}
	return fallback
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
