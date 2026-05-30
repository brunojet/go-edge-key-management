//go:build debug
// +build debug

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/brunojet/go-edge-key-management/internal/handler"
	"github.com/brunojet/go-edge-key-management/internal/rotator"
)

func main() {
	// CLI flags to drive rotation steps locally.
	// All flags are parsed manually to avoid importing "flag" into the non-debug build.
	step := flagStr("step", "createSecret")
	token := flagStr("token", "")
	secret := flagStr("secret", "")
	keyGroupName := flagStr("key-group-name", "")
	namePrefix := flagStr("name-prefix", "go-edge")
	runAll := flagBool("run-all", false)
	delay := flagInt("delay", 2)

	// Ensure ClientRequestToken meets the AWS Secrets Manager 32-char minimum.
	if *token == "" {
		*token = secureToken("debug")
	} else if len(*token) < 32 {
		log.Printf("token too short (%d chars), generating secure token", len(*token))
		*token = secureToken(*token)
	}

	// Export env vars consumed by rotator.Load().
	if *secret != "" {
		os.Setenv("SECRET_NAME", *secret)
	}
	os.Setenv("NAME_PREFIX", *namePrefix)
	if *keyGroupName != "" {
		os.Setenv("KEY_GROUP_NAME", *keyGroupName)
	} else {
		os.Setenv("KEY_GROUP_NAME", fmt.Sprintf("%s-key-group", *namePrefix))
	}

	h, err := handler.New(context.Background())
	if err != nil {
		log.Fatalf("init: %v", err)
	}

	evt := rotator.RotationEvent{
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
			if err := h.Handle(context.Background(), evt); err != nil {
				log.Fatalf("Handle error on %s: %v", s, err)
			}
			if i < len(steps)-1 {
				log.Printf("debug: sleeping %d seconds before next step", *delay)
				time.Sleep(time.Duration(*delay) * time.Second)
			}
		}
		return
	}

	if err := h.Handle(context.Background(), evt); err != nil {
		log.Fatalf("Handle error: %v", err)
	}
}
