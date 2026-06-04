# Upgrade to go-infra-adapters v4.0.0

## Breaking Changes

go-infra-adapters v4.0.0 introduces breaking changes to the crypto package interfaces.

### Changes Required

#### Before (v3.x):
```go
import "github.com/brunojet/go-infra-adapters/v3/pkg/crypto"

signer, _ := crypto.NewRSASignerFromPEM(privateKeyPEM)
signature, _ := signer.Sign(ctx, payload)  // ❌ No longer works
```

#### After (v4.0):
```go
import (
    "github.com/brunojet/go-infra-adapters/v4/pkg/crypto"
    "github.com/brunojet/go-infra-adapters/v4/pkg/crypto/contracts"
)

signer, _ := crypto.NewRSASignerFromPEM(privateKeyPEM)
// Must specify hash algorithm now:
signature, _ := signer.Sign(ctx, contracts.SHA256, payload)  // ✓ Required
```

### Supported Hash Algorithms

- `contracts.SHA256` — default, general-purpose signing
- `contracts.SHA1` — AWS CloudFront signed URLs
- `contracts.SHA512` — for high-security requirements

### CloudFront URL Signing

New dedicated package for CDN signing:

```go
import "github.com/brunojet/go-infra-adapters/v4/pkg/cdn"

signer, _ := cdn.NewCloudFrontSignerFromPEM(keyID, privateKeyPEM)
signedURL, _ := signer.SignURL(ctx, resourceURL, expiresAt)
```

## Update Steps

1. Update `go.mod`:
   ```bash
   go get github.com/brunojet/go-infra-adapters/v4@latest
   ```

2. Update import paths in code:
   ```
   v3 → v4
   ```

3. Update `Sign()` calls to include `contracts.SHA256`:
   ```go
   signer.Sign(ctx, contracts.SHA256, payload)
   ```

4. Update `Verify()` calls to include `contracts.SHA256`:
   ```go
   verifier.Verify(ctx, contracts.SHA256, payload, signature)
   ```

## Migration Checklist

- [ ] Update `go.mod` to v4.0.0
- [ ] Run `go mod tidy`
- [ ] Update import paths (v3 → v4)
- [ ] Add `contracts.SHA256` to all `Sign()` calls
- [ ] Add `contracts.SHA256` to all `Verify()` calls
- [ ] Run tests: `go test ./...`
- [ ] Verify build: `go build ./...`
