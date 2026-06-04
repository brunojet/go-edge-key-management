package rotator

import "fmt"

// validateEvent returns an error when the event is missing a ClientRequestToken,
// which is required by all four Secrets Manager rotation steps.
func validateEvent(event RotationEvent) error {
	if event.ClientRequestToken == "" {
		return fmt.Errorf("%s called without ClientRequestToken", event.Step)
	}
	return nil
}

// cdnKeyName computes the CDN public key name from the configured prefix and
// the key's fingerprint (first 8 hex chars of the SHA-256 of the public key).
func cdnKeyName(namePrefix, fingerprint string) string {
	suffix := fingerprint
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	return fmt.Sprintf("%s-%s", namePrefix, suffix)
}
