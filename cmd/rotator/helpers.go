// Package main implements the AWS Lambda handler for secret rotation.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"
)

// secureToken generates a token with cryptographic randomness.
// If generation fails, falls back to timestamp-based token.
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

// flagStr parses a string flag from os.Args.
// Returns default if flag not found.
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

// flagBool parses a boolean flag from os.Args.
// Flag presence sets it to true.
func flagBool(name string, def bool) *bool {
	v := def
	for _, arg := range os.Args[1:] {
		if arg == "--"+name || arg == "-"+name {
			v = true
		}
	}
	return &v
}

// flagInt parses an integer flag from os.Args.
// Returns default if flag not found or value is invalid.
func flagInt(name string, def int) *int {
	v := def
	for i, arg := range os.Args[1:] {
		if arg == "--"+name || arg == "-"+name {
			if i+1 < len(os.Args[1:]) {
				if parsed, err := strconv.Atoi(os.Args[i+2]); err == nil {
					v = parsed
				}
			}
		}
	}
	return &v
}
