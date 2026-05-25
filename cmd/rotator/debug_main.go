//go:build debug
// +build debug

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

func secureToken(seed string) string {
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

// flagStr / flagBool / flagInt are minimal flag parsers so that debug_main.go
// does not pull in the "flag" package at all in the non-debug build.
func flagStr(name, def string) *string {
	v := def
	for i, arg := range os.Args[1:] {
		if arg == "--"+name || arg == "-"+name {
			if i+1 < len(os.Args[1:]) {
				v = os.Args[i+2]
			}
		}
	}
	return &v
}

func flagBool(name string, def bool) *bool {
	v := def
	for _, arg := range os.Args[1:] {
		if arg == "--"+name || arg == "-"+name {
			v = true
		}
	}
	return &v
}

func flagInt(name string, def int) *int {
	v := def
	for i, arg := range os.Args[1:] {
		if arg == "--"+name || arg == "-"+name {
			if i+1 < len(os.Args[1:]) {
				fmt.Sscanf(os.Args[i+2], "%d", &v)
			}
		}
	}
	return &v
}
