package rotator

import (
	"os"
	"testing"
)

func TestLoad_Success(t *testing.T) {
	oldEnv := setEnv(map[string]string{
		"SECRET_NAME":                   "my-secret",
		"KEY_GROUP_NAME":                "my-key-group",
		"NAME_PREFIX":                   "custom-prefix",
		"MIN_ROTATION_INTERVAL_MINUTES": "120",
		"MAX_KEYS_IN_GROUP":             "5",
		"CLOUDFRONT_CONCURRENCY":        "10",
	})
	defer restoreEnv(oldEnv)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SecretName != "my-secret" {
		t.Errorf("SecretName: got %q, want %q", cfg.SecretName, "my-secret")
	}
	if cfg.KeyGroupName != "my-key-group" {
		t.Errorf("KeyGroupName: got %q, want %q", cfg.KeyGroupName, "my-key-group")
	}
	if cfg.NamePrefix != "custom-prefix" {
		t.Errorf("NamePrefix: got %q, want %q", cfg.NamePrefix, "custom-prefix")
	}
	if cfg.MinRotationIntervalMinutes != 120 {
		t.Errorf("MinRotationIntervalMinutes: got %d, want 120", cfg.MinRotationIntervalMinutes)
	}
	if cfg.MaxKeysInGroup != 5 {
		t.Errorf("MaxKeysInGroup: got %d, want 5", cfg.MaxKeysInGroup)
	}
	if cfg.CloudFrontConcurrency != 10 {
		t.Errorf("CloudFrontConcurrency: got %d, want 10", cfg.CloudFrontConcurrency)
	}
}

func TestLoad_Defaults(t *testing.T) {
	oldEnv := setEnv(map[string]string{
		"SECRET_NAME":    "my-secret",
		"KEY_GROUP_NAME": "my-key-group",
	})
	defer restoreEnv(oldEnv)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.NamePrefix != defaultNamePrefix {
		t.Errorf("NamePrefix: got %q, want %q", cfg.NamePrefix, defaultNamePrefix)
	}
	if cfg.MinRotationIntervalMinutes != 60 {
		t.Errorf("MinRotationIntervalMinutes: got %d, want 60", cfg.MinRotationIntervalMinutes)
	}
	if cfg.MaxKeysInGroup != 3 {
		t.Errorf("MaxKeysInGroup: got %d, want 3", cfg.MaxKeysInGroup)
	}
	if cfg.CloudFrontConcurrency != 5 {
		t.Errorf("CloudFrontConcurrency: got %d, want 5", cfg.CloudFrontConcurrency)
	}
}

func TestLoad_MissingSecretName(t *testing.T) {
	oldEnv := setEnv(map[string]string{
		"KEY_GROUP_NAME": "my-key-group",
	})
	defer restoreEnv(oldEnv)

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing SECRET_NAME")
	}
	if err.Error() != "SECRET_NAME must be set in environment" {
		t.Errorf("got error %q, want 'SECRET_NAME must be set in environment'", err.Error())
	}
}

func TestLoad_MissingKeyGroupName(t *testing.T) {
	oldEnv := setEnv(map[string]string{
		"SECRET_NAME": "my-secret",
	})
	defer restoreEnv(oldEnv)

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing KEY_GROUP_NAME")
	}
	if err.Error() != "KEY_GROUP_NAME must be set in environment" {
		t.Errorf("got error %q, want 'KEY_GROUP_NAME must be set in environment'", err.Error())
	}
}

func TestLoad_NegativeMinRotationInterval(t *testing.T) {
	oldEnv := setEnv(map[string]string{
		"SECRET_NAME":                   "my-secret",
		"KEY_GROUP_NAME":                "my-key-group",
		"MIN_ROTATION_INTERVAL_MINUTES": "-1",
	})
	defer restoreEnv(oldEnv)

	_, err := Load()
	if err == nil {
		t.Error("expected error for negative MIN_ROTATION_INTERVAL_MINUTES")
	}
	if err.Error() != "MIN_ROTATION_INTERVAL_MINUTES must be >= 0" {
		t.Errorf("got error %q, want 'MIN_ROTATION_INTERVAL_MINUTES must be >= 0'", err.Error())
	}
}

func TestLoad_InvalidMaxKeysInGroup(t *testing.T) {
	oldEnv := setEnv(map[string]string{
		"SECRET_NAME":       "my-secret",
		"KEY_GROUP_NAME":    "my-key-group",
		"MAX_KEYS_IN_GROUP": "0",
	})
	defer restoreEnv(oldEnv)

	_, err := Load()
	if err == nil {
		t.Error("expected error for MAX_KEYS_IN_GROUP < 1")
	}
	if err.Error() != "MAX_KEYS_IN_GROUP must be >= 1" {
		t.Errorf("got error %q, want 'MAX_KEYS_IN_GROUP must be >= 1'", err.Error())
	}
}

func TestLoad_InvalidCloudFrontConcurrency(t *testing.T) {
	oldEnv := setEnv(map[string]string{
		"SECRET_NAME":            "my-secret",
		"KEY_GROUP_NAME":         "my-key-group",
		"CLOUDFRONT_CONCURRENCY": "0",
	})
	defer restoreEnv(oldEnv)

	_, err := Load()
	if err == nil {
		t.Error("expected error for CLOUDFRONT_CONCURRENCY < 1")
	}
	if err.Error() != "CLOUDFRONT_CONCURRENCY must be >= 1" {
		t.Errorf("got error %q, want 'CLOUDFRONT_CONCURRENCY must be >= 1'", err.Error())
	}
}

func TestEnvOrDefaultInt_WithValue(t *testing.T) {
	oldEnv := map[string]string{
		"TEST_INT": os.Getenv("TEST_INT"),
	}
	_ = os.Setenv("TEST_INT", "42")
	defer func() { _ = os.Setenv("TEST_INT", oldEnv["TEST_INT"]) }()

	got := envOrDefaultInt("TEST_INT", 10)
	if got != 42 {
		t.Errorf("envOrDefaultInt: got %d, want 42", got)
	}
}

func TestEnvOrDefaultInt_WithDefault(t *testing.T) {
	oldEnv := map[string]string{
		"TEST_INT_NONEXIST": os.Getenv("TEST_INT_NONEXIST"),
	}
	_ = os.Unsetenv("TEST_INT_NONEXIST")
	defer func() {
		if oldEnv["TEST_INT_NONEXIST"] != "" {
			_ = os.Setenv("TEST_INT_NONEXIST", oldEnv["TEST_INT_NONEXIST"])
		}
	}()

	got := envOrDefaultInt("TEST_INT_NONEXIST", 10)
	if got != 10 {
		t.Errorf("envOrDefaultInt: got %d, want 10", got)
	}
}

func TestEnvOrDefaultInt_InvalidValue(t *testing.T) {
	oldEnv := map[string]string{
		"TEST_INT_INVALID": os.Getenv("TEST_INT_INVALID"),
	}
	_ = os.Setenv("TEST_INT_INVALID", "not-a-number")
	defer func() { _ = os.Setenv("TEST_INT_INVALID", oldEnv["TEST_INT_INVALID"]) }()

	got := envOrDefaultInt("TEST_INT_INVALID", 10)
	if got != 10 {
		t.Errorf("envOrDefaultInt with invalid value: got %d, want 10", got)
	}
}

func TestEnvOrDefault_WithValue(t *testing.T) {
	oldEnv := map[string]string{
		"TEST_STR": os.Getenv("TEST_STR"),
	}
	_ = os.Setenv("TEST_STR", "value")
	defer func() { _ = os.Setenv("TEST_STR", oldEnv["TEST_STR"]) }()

	got := envOrDefault("TEST_STR", "default")
	if got != "value" {
		t.Errorf("envOrDefault: got %q, want %q", got, "value")
	}
}

func TestEnvOrDefault_WithDefault(t *testing.T) {
	oldEnv := map[string]string{
		"TEST_STR_NONEXIST": os.Getenv("TEST_STR_NONEXIST"),
	}
	_ = os.Unsetenv("TEST_STR_NONEXIST")
	defer func() {
		if oldEnv["TEST_STR_NONEXIST"] != "" {
			_ = os.Setenv("TEST_STR_NONEXIST", oldEnv["TEST_STR_NONEXIST"])
		}
	}()

	got := envOrDefault("TEST_STR_NONEXIST", "default")
	if got != "default" {
		t.Errorf("envOrDefault: got %q, want %q", got, "default")
	}
}

// Helper functions
func setEnv(vars map[string]string) map[string]string {
	old := make(map[string]string)
	for key := range vars {
		old[key] = os.Getenv(key)
		_ = os.Unsetenv(key)
	}
	for key, val := range vars {
		_ = os.Setenv(key, val)
	}
	return old
}

func restoreEnv(old map[string]string) {
	for key := range old {
		_ = os.Unsetenv(key)
	}
	for key, val := range old {
		if val != "" {
			_ = os.Setenv(key, val)
		}
	}
}
