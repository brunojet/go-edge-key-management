package rotator

import (
	"fmt"
	"os"
	"strconv"
)

const (
	defaultNamePrefix = "go-edge"
)

// Config holds every parameter the rotator needs to run.
// All fields are populated from environment variables via Load.
type Config struct {
	// SecretName is the Secrets Manager secret that holds the rotation payload.
	SecretName string

	// KeyGroupName, when provided, is used as the KeyGroup's Name when
	// creating a new KeyGroup. `KeyGroupID` takes precedence over this value.
	KeyGroupName string

	// NamePrefix is the prefix used when naming CloudFront resources
	// (public keys, key groups).
	NamePrefix string

	// MinRotationIntervalMinutes prevents rotations that occur more frequently
	// than the configured interval (in minutes). When 0 the check is disabled.
	MinRotationIntervalMinutes int

	// MaxKeysInGroup is the maximum number of public keys kept active in the
	// CloudFront KeyGroup at any time. Older keys beyond this limit are evicted.
	MaxKeysInGroup int

	// CloudFrontConcurrency bounds the number of parallel CloudFront API calls
	// issued during key metadata fetches and orphan-key deletions.
	CloudFrontConcurrency int
}

// Load reads configuration from environment variables and applies defaults.
// Returns an error when a required variable is absent or invalid.
func Load() (Config, error) {
	cfg := Config{
		SecretName:                 os.Getenv("SECRET_NAME"),
		KeyGroupName:               os.Getenv("KEY_GROUP_NAME"),
		NamePrefix:                 envOrDefault("NAME_PREFIX", defaultNamePrefix),
		MinRotationIntervalMinutes: envOrDefaultInt("MIN_ROTATION_INTERVAL_MINUTES", 60),
		MaxKeysInGroup:             envOrDefaultInt("MAX_KEYS_IN_GROUP", 3),
		CloudFrontConcurrency:      envOrDefaultInt("CLOUDFRONT_CONCURRENCY", 5),
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) validate() error {
	if c.SecretName == "" {
		return fmt.Errorf("SECRET_NAME must be set in environment")
	}
	if c.KeyGroupName == "" {
		return fmt.Errorf("KEY_GROUP_NAME must be set in environment")
	}
	if c.MinRotationIntervalMinutes < 0 {
		return fmt.Errorf("MIN_ROTATION_INTERVAL_MINUTES must be >= 0")
	}
	if c.MaxKeysInGroup < 1 {
		return fmt.Errorf("MAX_KEYS_IN_GROUP must be >= 1")
	}
	if c.CloudFrontConcurrency < 1 {
		return fmt.Errorf("CLOUDFRONT_CONCURRENCY must be >= 1")
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

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
