# Architecture: go-edge-key-management

Lambda-based RSA key rotation for CloudFront Signed URLs.

---

## Overview

**Problem:** CloudFront Signed URLs require keypairs. Keys must rotate periodically without manual intervention.

**Solution:** Secrets Manager automation invokes Lambda handler, which orchestrates key rotation across Secrets Manager + CloudFront.

---

## Flow: 4-Step Rotation

```
1. Secrets Manager detects rotation needed (time-based trigger)
2. Lambda receives RotationEvent with Step + ClientRequestToken
3. Lambda handles step → updates state in Secrets Manager

Automation repeats until all 4 steps complete
```

### Step 1: createSecret
- Generate RSA 2048-bit keypair
- Create public key in CloudFront (returns publicKeyID)
- Store {privateKey, publicKey, fingerprint, publicKeyID} as AWSPENDING version
- **Cleanup:** If create fails → remove orphan public key

### Step 2: setSecret
- Fetch AWSPENDING version
- Add publicKeyID to CloudFront KeyGroup
- KeyGroup maintains `MaxKeysInGroup` (default: 3) — evicts oldest if at limit
- **Cleanup:** If group update fails → remove public key + abort

### Step 3: testSecret
- Fetch AWSPENDING version
- Verify publicKeyID is visible in CloudFront KeyGroup
- Reject if not visible (prevents timing gap issues)
- **Cleanup:** If verify fails → remove public key + abort

### Step 4: finishSecret
- Promote AWSPENDING → AWSCURRENT in Secrets Manager
- Idempotent: clientRequestToken ensures no double-promotion
- Old AWSCURRENT → AWSPREVIOUS (Secrets Manager handles)

---

## Components

### Lambda (Handler)
- Runtime: `provided.al2` (custom Go runtime)
- Architecture: ARM64
- Timeout: 60s
- Memory: 256 MB (configurable)

**Code:**
- `cmd/rotator/main.go` — Lambda entry point
- `internal/handler/handler.go` — AWS client wiring + health checks
- `internal/rotator/service.go` — 4-step rotation orchestration
- `internal/rotator/config.go` — environment-based config

**Invocation:** Automatic via Secrets Manager rotation event. No manual trigger needed.

### Secrets Manager
- Stores secret: `{private_key, public_key, fingerprint, key_group_name, public_key_id}`
- Manages 2 active versions: `AWSCURRENT` + `AWSPENDING` (Secrets Manager auto-cleanup)
- Rotation policy: `RotationRules { AutomaticallyAfterDays: 7 }` (configurable)
- Each version immutable by `sysid` (RSA fingerprint ensures uniqueness)

### CloudFront
- PublicKey: stores RSA public key (immutable once created, identified by AWS-assigned ID)
- KeyGroup: ordered list of PublicKey IDs (current, previous, maybe older)
  - Managed dynamically by Lambda
  - Max size: `MaxKeysInGroup` (default 3, tunable)

### IAM Role (Lambda)
Least-privilege:
- **Secrets Manager:** GetSecretValue, PutSecretValue, DescribeSecret (secret ARN only)
- **CloudFront:** CreatePublicKey, GetPublicKey, DeletePublicKey, CreateKeyGroup, GetKeyGroup, UpdateKeyGroup
- **CloudWatch:** CreateLogStream, PutLogEvents (log group only)

### Terraform Module
Path: `terraform/modules/key_rotator/`

**Resources:**
- Lambda function (code, role, log group)
- IAM role + policies
- CloudWatch log group (14-day retention)
- Secrets Manager secret + rotation rule
- CloudFront: none (created dynamically by Lambda)

**Inputs:**
- `secret_name` — Secrets Manager secret ARN/name
- `key_group_name` — CloudFront key group name (Lambda uses this)
- `lambda_zip` — path to built Lambda zip
- `rotation_days` — rotation interval (default 7)
- `min_rotation_interval_minutes` — minimum time between rotations (default 60)
- `max_keys_in_group` — max keys in CloudFront KeyGroup (default 3)
- `log_retention_days` — CloudWatch log retention (default 14)

---

## Key Design Decisions

### 1. Secrets Manager Rotation Contract
Use native Secrets Manager rotation (4-step) instead of custom scheduler (EventBridge/SQS).
- **Why:** AWS-managed scheduling, built-in retry logic, atomicity
- **Trade-off:** Bound to Secrets Manager event model (less flexible, but simpler)

### 2. Dynamic Key Group Management
CloudFront KeyGroup created/managed by Lambda, not Terraform.
- **Why:** KeyGroup ID needed at runtime; Terraform can't inject into rotation event
- **Trade-off:** Terraform does not provision KeyGroup (lower IaC coverage, but simpler code)

### 3. ARM64 Architecture
Lambda runs on ARM64 (Graviton2).
- **Why:** Better cost/performance ratio, native support for provided.al2
- **Trade-off:** Requires ARM64-compatible build (Go supports this natively)

### 4. MinRotationInterval
Default 60 minutes between rotations.
- **Why:** Prevents API flood; CloudFront rate limits ~1000 ops/min
- **Trade-off:** Short-interval rotations blocked (acceptable for daily schedule)

### 5. Max Keys in Group
Default 3 keys (current + previous + buffer).
- **Why:** Accommodates slow client cache expiry; prevents orphan keys
- **Trade-off:** More keys = longer CloudFront API calls, slightly higher latency

---

## Error Handling & Idempotency

Each step is **idempotent**: re-invoking with same `clientRequestToken` is safe.

```
createSecret      → sanitizes orphan keys on failure
setSecret         → sanitizes orphan keys + aborts on failure
testSecret        → sanitizes orphan keys + aborts if not visible
finishSecret      → idempotent (token-based, no sanitization needed)
```

**Result:** Partial failures roll back cleanly; retries converge to success.

---

## Monitoring & Observability

CloudWatch logs structured (slog):
```json
{
  "time": "2026-06-06T12:34:56Z",
  "level": "INFO",
  "msg": "createSecret: public key created",
  "keyID": "K1234567...",
  "version": "token-uuid"
}
```

**Metrics to track:**
- Lambda duration (per step)
- Rotation success/failure rate
- CloudFront API latency
- Key group size (keys in active group)

---

## Deployment

### Build
```bash
./scripts/build-lambda.sh
# Creates: rotator.zip (Lambda code + dependencies)
```

### Provision
```bash
cd terraform/env/dev
terraform init
terraform apply -var-file=terraform.tfvars
```

### Verify
1. Check CloudWatch logs for rotation completion
2. Manually check CloudFront key group: AWS Console → CloudFront → Key groups
3. Optional: Trigger manual rotation (Secrets Manager → secret → rotate now)

---

## FAQ

**Q: Can I disable rotation?**
A: Set `rotation_days = 0` in Terraform (disables automatic rotation). Manual rotation still possible.

**Q: What if rotation fails?**
A: Check CloudWatch logs. Most failures auto-retry (Secrets Manager retry policy). See FAILURE_RUNBOOK.md.

**Q: How many keys should I keep?**
A: Default 3 is safe. Increase only if clients have very long cache TTL (> 24h).

**Q: Can I use the same KeyGroup for multiple secrets?**
A: Yes, but not recommended (one secret = one key rotation schedule). Use separate KeyGroups for different secrets.

**Q: What's the cost impact?**
A: Lambda: ~$0.20/month (once daily). CloudFront API: ~$1-2/month. Secrets Manager: $0.40/secret/month.
