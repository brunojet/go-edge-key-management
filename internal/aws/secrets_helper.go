package awsclient

import (
	"encoding/json"
	"fmt"
)

func marshal[T any](v *T) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return b, nil
}

func unmarshal[T any](s string) (*T, error) {
	var v T
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	return &v, nil
}
