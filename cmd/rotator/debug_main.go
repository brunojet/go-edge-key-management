//go:build debug
// +build debug

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	// CLI flags to drive rotation steps locally.
	step := flag.String("step", "createSecret", "rotation step: createSecret|setSecret|testSecret|finishSecret")
	token := flag.String("token", "", "ClientRequestToken to use (default: generated)")
	secret := flag.String("secret", "", "Secrets Manager secret name or ARN (optional)")
	keyGroupName := flag.String("key-group-name", "", "CloudFront KeyGroup name (optional)")
	namePrefix := flag.String("name-prefix", "go-edge", "resource name prefix")
	keyRetentionDays := flag.Int("key-retention-days", 30, "key retention days")
	minPublicKeys := flag.Int("min-public-keys-to-keep", 2, "minimum public keys to keep")
	onlyDeleteManaged := flag.Bool("only-delete-managed-keys", true, "only delete managed keys")
	runAll := flag.Bool("run-all", false, "run all rotation steps in sequence: createSecret,setSecret,testSecret,finishSecret")
	delay := flag.Int("delay", 2, "seconds to wait between steps when running full sequence")

	flag.Parse()

	// Ensure ClientRequestToken meets AWS Secrets Manager constraint
	// (minimum length 32). If the provided token is empty or too short,
	// generate a secure random token (hex-encoded 16 bytes = 32 chars).
	makeSecureToken := func(seed string) string {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return fmt.Sprintf("%s-%d", seed, time.Now().UnixNano())
		}
		h := hex.EncodeToString(b)
		if seed != "" {
			return fmt.Sprintf("%s-%s", seed, h)
		}
		return h
	}

	if *token == "" {
		*token = makeSecureToken("debug")
	} else if len(*token) < 32 {
		log.Printf("provided token too short (%d chars), generating secure token", len(*token))
		*token = makeSecureToken(*token)
	}

	// Export env vars consumed by rotator.Load(). Prefer explicit flags,
	// otherwise derive sensible defaults for local debugging.
	if *secret != "" {
		os.Setenv("SECRET_NAME", *secret)
	}
	os.Setenv("NAME_PREFIX", *namePrefix)
	if *keyGroupName != "" {
		os.Setenv("KEY_GROUP_NAME", *keyGroupName)
	} else {
		// Derive a default KeyGroup name for local debugging so Load() passes validation.
		os.Setenv("KEY_GROUP_NAME", fmt.Sprintf("%s-key-group", *namePrefix))
	}
	os.Setenv("KEY_RETENTION_DAYS", fmt.Sprintf("%d", *keyRetentionDays))
	os.Setenv("MIN_PUBLIC_KEYS_TO_KEEP", fmt.Sprintf("%d", *minPublicKeys))
	os.Setenv("ONLY_DELETE_MANAGED_KEYS", fmt.Sprintf("%t", *onlyDeleteManaged))

	// Build rotation event used for single-step or sequential execution.
	evt := rotationEvent{
		Step:               *step,
		SecretId:           os.Getenv("SECRET_NAME"),
		ClientRequestToken: *token,
	}

	log.Printf("debug: step=%s token=%s secret=%s keygroup=%s nameprefix=%s",
		evt.Step, evt.ClientRequestToken, evt.SecretId, os.Getenv("KEY_GROUP_NAME"), os.Getenv("NAME_PREFIX"))

	if *runAll {
		steps := []string{"createSecret", "setSecret", "testSecret", "finishSecret"}
		for i, s := range steps {
			evt.Step = s
			log.Printf("debug: running step %d/%d: %s", i+1, len(steps), s)
			if err := RotationHandler(context.Background(), evt); err != nil {
				log.Fatalf("RotationHandler error on %s: %v", s, err)
			}
			if i < len(steps)-1 {
				log.Printf("debug: sleeping %d seconds before next step", *delay)
				time.Sleep(time.Duration(*delay) * time.Second)
			}
		}
		return
	}

	if err := RotationHandler(context.Background(), evt); err != nil {
		log.Fatalf("RotationHandler error: %v", err)
	}
}
