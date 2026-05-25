package awsclient

import (
	"errors"
	"strings"

	cfTypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	smTypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

// IsStillInUse reports whether err indicates the CloudFront resource cannot be
// deleted because it is still referenced by another resource.
func IsStillInUse(err error) bool {
	if err == nil {
		return false
	}
	var pub *cfTypes.PublicKeyInUse
	if errors.As(err, &pub) {
		return true
	}
	var entity *cfTypes.CannotDeleteEntityWhileInUse
	if errors.As(err, &entity) {
		return true
	}
	var res *cfTypes.ResourceInUse
	return errors.As(err, &res)
}

// IsNotFound reports whether err represents a "resource not found" response
// from CloudFront or Secrets Manager. Typed SDK errors are checked first;
// the string fallback covers test mocks and future SDK changes.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	var smNF *smTypes.ResourceNotFoundException
	if errors.As(err, &smNF) {
		return true
	}
	var cfNoKey *cfTypes.NoSuchPublicKey
	if errors.As(err, &cfNoKey) {
		return true
	}
	// Fallback for non-typed or wrapped errors (e.g. test mocks, future SDK changes).
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "resourcenotfoundexception") ||
		strings.Contains(msg, "nosuchpublickey")
}
