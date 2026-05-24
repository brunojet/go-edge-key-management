package awsclient

import (
	"context"
	"errors"
	"fmt"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	sdkretry "github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/smithy-go"
)

// CustomRetryer wraps an SDK retryer and treats a small set of additional
// smithy API error codes as retryable. It delegates token/spacing behavior to
// the underlying standard retryer.
type CustomRetryer struct {
	base        awssdk.Retryer
	retryCodes  map[string]struct{}
	maxAttempts int
}

// NewRetryerProvider returns a provider function suitable for assigning to
// aws.Config.Retryer. The returned provider will create a retryer that
// delegates most behavior to the SDK's Standard retryer, but also marks any
// smithy API error with a code present in extraCodes as retryable.
func NewRetryerProvider(extraCodes []string, maxAttempts int) func() awssdk.Retryer {
	codes := make(map[string]struct{}, len(extraCodes))
	for _, c := range extraCodes {
		codes[c] = struct{}{}
	}

	return func() awssdk.Retryer {
		return &CustomRetryer{
			base:        sdkretry.NewStandard(),
			retryCodes:  codes,
			maxAttempts: maxAttempts,
		}
	}
}

func (r *CustomRetryer) IsErrorRetryable(err error) bool {
	if err == nil {
		return false
	}
	// Base retryer decides first.
	if r.base != nil && r.base.IsErrorRetryable(err) {
		return true
	}
	// Check for smithy APIError codes explicitly configured as retryable.
	var aerr smithy.APIError
	if errors.As(err, &aerr) {
		if _, ok := r.retryCodes[aerr.ErrorCode()]; ok {
			return true
		}
	}
	return false
}

func (r *CustomRetryer) MaxAttempts() int {
	if r.maxAttempts != 0 {
		return r.maxAttempts
	}
	if r.base != nil {
		return r.base.MaxAttempts()
	}
	return 0
}

func (r *CustomRetryer) RetryDelay(attempt int, opErr error) (time.Duration, error) {
	if r.base != nil {
		return r.base.RetryDelay(attempt, opErr)
	}
	return 0, fmt.Errorf("no underlying retryer")
}

func (r *CustomRetryer) GetRetryToken(ctx context.Context, opErr error) (func(error) error, error) {
	if r.base != nil {
		return r.base.GetRetryToken(ctx, opErr)
	}
	return func(error) error { return nil }, nil
}

func (r *CustomRetryer) GetInitialToken() func(error) error {
	if r.base != nil {
		return r.base.GetInitialToken()
	}
	return func(error) error { return nil }
}
