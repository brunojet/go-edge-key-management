# Failure Runbook: Key Rotation Diagnostics & Recovery

---

## Quick Diagnosis

When rotation fails, check in order:

### 1. Check CloudWatch Logs
```bash
# View latest logs (last 5min)
aws logs tail /aws/lambda/key-rotator --follow --since 5m
```

Look for error messages. Common patterns:
- `"ensure key group"` → CloudFront API issue
- `"generate key pair"` → RSA generation error (unlikely)
- `"verify pending"` → Key not visible in group
- `"finish secret"` → Secrets Manager promotion failed

### 2. Check Lambda Metrics
```bash
# AWS Console → CloudWatch → Metrics → Lambda
# Or CLI:
aws cloudwatch get-metric-statistics \
  --namespace AWS/Lambda \
  --metric-name Duration \
  --start-time 2026-06-06T00:00:00Z \
  --end-time 2026-06-06T23:59:59Z \
  --period 3600 \
  --statistics Average,Maximum \
  --dimensions Name=FunctionName,Value=key-rotator
```

Look for: timeouts (should be rare), high error rates.

### 3. Check Secrets Manager
```bash
# List all versions
aws secretsmanager list-secret-version-ids \
  --secret-id /go-edge-key-management/rotator

# Get current version
aws secretsmanager get-secret-value \
  --secret-id /go-edge-key-management/rotator \
  --version-stage AWSCURRENT
```

Look for: stuck AWSPENDING version (indicates incomplete rotation).

### 4. Check CloudFront KeyGroup
```bash
# List key groups
aws cloudfront list-key-groups

# Get key group details (use KeyGroupId from above)
aws cloudfront get-key-group --id K1234567...

# List public keys
aws cloudfront list-public-keys
```

Look for: orphan keys (public keys not in any key group).

---

## Failure Scenarios & Recovery

### Scenario 1: createSecret Failed

**Symptom:** No AWSPENDING version created. Logs show "generate key pair" error.

**Why:** RSA generation failed (very rare — usually system resource issue).

**Recovery:**
1. Wait 5 minutes (Secrets Manager will retry)
2. If persists, check Lambda memory/timeout, increase to 512MB
3. Manual retry: `aws secretsmanager rotate-secret --secret-id /go-edge-key-management/rotator`

---

### Scenario 2: setSecret Failed (KeyGroup Update Failed)

**Symptom:** AWSPENDING exists, but public key not in KeyGroup. Logs show "ensure key group" error.

**Why:** CloudFront API error (throttle, permission, or transient).

**Recovery:**
1. Check CloudFront API limits: `aws cloudfront get-public-key --id [publicKeyID]` (should exist)
2. Check KeyGroup: `aws cloudfront get-key-group --id [keyGroupId]` (public key should NOT be there)
3. Public key was auto-sanitized (removed) on failure ✅
4. Retry: `aws secretsmanager rotate-secret --secret-id /go-edge-key-management/rotator`

**If manual cleanup needed:**
```bash
# Remove orphan public key (if still present)
aws cloudfront delete-public-key --id [publicKeyID]
```

---

### Scenario 3: testSecret Failed (Key Not Visible in Group)

**Symptom:** Public key created + added to group, but verification fails. Logs show "verify pending" error.

**Why:** Timing gap (key not yet visible in CloudFront cache), or group not actually updated.

**Recovery:**
1. Wait 30 seconds (CloudFront cache sync)
2. Manual verify:
   ```bash
   aws cloudfront get-key-group --id [keyGroupId]
   # Check if your public key ID is in "KeyGroupConfig.Items"
   ```
3. If key IS there now → retry. Retry will pass ✅
4. If key NOT there → setSecret failed silently (very rare). Check permissions.

---

### Scenario 4: finishSecret Failed (Version Promotion Failed)

**Symptom:** AWSPENDING stuck. Logs show "finish secret" error.

**Why:** Secrets Manager API error (permission, quota, or transient).

**Recovery:**
1. Check secret version state:
   ```bash
   aws secretsmanager list-secret-version-ids \
     --secret-id /go-edge-key-management/rotator
   ```
   Look for AWSPENDING version.

2. Manual promotion (idempotent):
   ```bash
   aws secretsmanager put-secret-value \
     --secret-id /go-edge-key-management/rotator \
     --client-request-token [TOKEN_FROM_AWSPENDING] \
     --secret-string [SECRET_VALUE_FROM_AWSPENDING] \
     --version-stages AWSCURRENT
   ```

3. Verify AWSCURRENT updated:
   ```bash
   aws secretsmanager get-secret-value \
     --secret-id /go-edge-key-management/rotator \
     --version-stage AWSCURRENT
   ```

---

### Scenario 5: Rotation Looping (Multiple Invocations in Short Time)

**Symptom:** CloudWatch shows 10+ Lambda invocations in 1 hour. Logs repeat same errors.

**Why:** AWSPENDING stuck + MinRotationInterval bypass.

**Recovery:**
1. Check AWSPENDING age:
   ```bash
   aws secretsmanager describe-secret \
     --secret-id /go-edge-key-management/rotator
   ```
   Look at version timestamps.

2. If AWSPENDING is > MinRotationInterval old (default 60min):
   ```bash
   # Force cleanup + new rotation
   aws secretsmanager rotate-secret \
     --secret-id /go-edge-key-management/rotator \
     --force-rotate-without-validation
   ```

3. Monitor logs: rotation should succeed within 60s.

---

### Scenario 6: Orphan Keys Accumulating (KeyGroup Size Growing)

**Symptom:** CloudFront KeyGroup has 5+ keys. Expect: 3 max.

**Why:** Keys not evicted properly, or rotations happening > MinRotationInterval.

**Recovery:**
1. List keys in group:
   ```bash
   aws cloudfront get-key-group --id [keyGroupId] | jq '.KeyGroupConfig.Items[]'
   ```

2. Identify unused keys (consult last 10 rotations in CloudWatch).

3. Manual cleanup:
   ```bash
   # Remove old key (must not be referenced by distributions!)
   aws cloudfront delete-public-key --id [oldKeyID]
   
   # Update key group to remove reference
   aws cloudfront update-key-group \
     --id [keyGroupId] \
     --key-group-config '{"Items":["key1","key2","key3"]}'
   ```

4. Verify rotation cleans up next time (should evict if at max).

---

### Scenario 7: Permission Denied (IAM Errors)

**Symptom:** Logs show `AccessDenied` or `UnauthorizedOperation`.

**Why:** Lambda role missing permission or policy attached to wrong resource.

**Recovery:**
1. Check Lambda role:
   ```bash
   aws iam get-role --role-name key-rotator-role | jq '.Role.Arn'
   ```

2. Verify policies attached:
   ```bash
   aws iam list-attached-role-policies \
     --role-name key-rotator-role
   ```

3. Check inline policies:
   ```bash
   aws iam list-role-policies --role-name key-rotator-role
   ```

4. Verify policy includes correct ARNs for secret + key group.

5. Re-apply Terraform:
   ```bash
   terraform apply -var-file=terraform.tfvars
   ```

---

## Proactive Monitoring

Set up CloudWatch alarms (manual for now, see OPERATIONS.md):

```
Alarm 1: Lambda errors/5min > 0
  → Notify on-call immediately

Alarm 2: Lambda duration > 30s
  → Indicates slower CloudFront APIs or timeout risk

Alarm 3: Rotation interval < MinRotationInterval
  → Indicates configuration drift or bug
```

---

## Escalation

If rotation fails and recovery steps above don't work:

1. **Collect logs:**
   ```bash
   aws logs get-log-events \
     --log-group-name /aws/lambda/key-rotator \
     --log-stream-name [LATEST_STREAM] \
     --start-time [ERROR_TIME_MS] \
     > /tmp/rotation_failure.json
   ```

2. **Dump state:**
   ```bash
   aws secretsmanager describe-secret \
     --secret-id /go-edge-key-management/rotator \
     > /tmp/secret_state.json
   
   aws cloudfront list-key-groups \
     > /tmp/keygroups.json
   ```

3. **Check CloudFront account limits:**
   ```bash
   aws cloudfront get-account-summary
   ```

4. **Report:** Include logs + state dumps when opening AWS Support case.

---

## Prevention Checklist

- [ ] MinRotationInterval set to ≥ 60 minutes (default)
- [ ] MaxKeysInGroup ≥ 2, ≤ 10 (default 3)
- [ ] CloudWatch logs retention ≥ 7 days
- [ ] Lambda timeout ≥ 60 seconds
- [ ] Lambda memory ≥ 256 MB
- [ ] IAM role attached to Lambda
- [ ] Secret + KeyGroup names match config
- [ ] Test rotation manually once per month
