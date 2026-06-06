# Operations: Monitoring, Maintenance & Procedures

---

## CloudWatch Monitoring

### Logs
- **Location:** `/aws/lambda/key-rotator`
- **Retention:** 14 days (configurable via `log_retention_days`)
- **Format:** Structured JSON (slog)

**Sample query (CloudWatch Logs Insights):**
```
fields @timestamp, @message, @duration
| filter @message like /createSecret|setSecret|testSecret|finishSecret/
| stats count() as invocations, avg(@duration) as avg_ms by @message
```

### Metrics
- **Namespace:** `AWS/Lambda`
- **Dimensions:** `FunctionName=key-rotator`

**Key metrics:**
| Metric | Threshold | Action |
|--------|-----------|--------|
| Errors | > 0 per rotation | Page on-call |
| Duration | > 30s | Investigate slow APIs |
| Throttles | > 0 | Check CloudFront rate limits |
| ConcurrentExecutions | > 1 | Unexpected (should be ≤ 1) |

### Alarms (Manual Setup)

**Alarm 1: Rotation Failed**
```bash
aws cloudwatch put-metric-alarm \
  --alarm-name key-rotator-errors \
  --alarm-description "Lambda rotation failed" \
  --metric-name Errors \
  --namespace AWS/Lambda \
  --statistic Sum \
  --period 300 \
  --threshold 1 \
  --comparison-operator GreaterThanOrEqualToThreshold \
  --evaluation-periods 1 \
  --dimensions Name=FunctionName,Value=key-rotator
```

**Alarm 2: Slow Rotation**
```bash
aws cloudwatch put-metric-alarm \
  --alarm-name key-rotator-slow \
  --alarm-description "Lambda rotation slow (>30s)" \
  --metric-name Duration \
  --namespace AWS/Lambda \
  --statistic Maximum \
  --period 300 \
  --threshold 30000 \
  --comparison-operator GreaterThanThreshold \
  --evaluation-periods 1 \
  --dimensions Name=FunctionName,Value=key-rotator
```

**Future:** Terraform module should define alarms + SNS subscriptions.

---

## Manual Rotation

Trigger rotation outside scheduled time:

```bash
aws secretsmanager rotate-secret \
  --secret-id /go-edge-key-management/rotator
```

Check progress:
```bash
# Watch logs
aws logs tail /aws/lambda/key-rotator --follow

# Or poll secret state
watch -n 10 'aws secretsmanager describe-secret --secret-id /go-edge-key-management/rotator | jq .RotationEnabled'
```

---

## Configuration Tuning

### Environment Variables (Lambda)
Set via Terraform variables → Lambda environment block.

| Variable | Default | Range | Notes |
|----------|---------|-------|-------|
| `SECRET_NAME` | (required) | N/A | Secrets Manager secret |
| `KEY_GROUP_NAME` | (required) | N/A | CloudFront key group name |
| `NAME_PREFIX` | `go-edge` | Any | Used for naming public keys |
| `MIN_ROTATION_INTERVAL_MINUTES` | 60 | ≥ 0 | Prevents rapid rotations |
| `MAX_KEYS_IN_GROUP` | 3 | 2-10 | Current + previous + buffer |
| `CLOUDFRONT_CONCURRENCY` | 5 | 1-10 | Parallel API calls |

### Terraform Variables
Path: `env/dev/terraform.tfvars`

| Variable | Default | Purpose |
|----------|---------|---------|
| `rotation_days` | 7 | How often to rotate (days) |
| `lambda_memory` | 256 | MB allocated to Lambda |
| `lambda_timeout` | 60 | Seconds before timeout |
| `log_retention_days` | 14 | CloudWatch log retention |

---

## Maintenance Tasks

### Monthly: Test Rotation
Ensure automation still works:
```bash
aws secretsmanager rotate-secret \
  --secret-id /go-edge-key-management/rotator \
  --rotate-immediately
```
Watch CloudWatch logs. Should complete in <30s.

### Monthly: Audit Key Group
Check for orphan keys:
```bash
aws cloudfront get-key-group --id [keyGroupId] | jq '.KeyGroupConfig.Items | length'
# Expected: ≤ MAX_KEYS_IN_GROUP (default 3)
```

### Quarterly: Review Logs
Scan for unexpected errors:
```bash
aws logs filter-log-events \
  --log-group-name /aws/lambda/key-rotator \
  --start-time $(($(date +%s) - 86400*90))000 \
  --filter-pattern "ERROR"
```

### Yearly: Rotate Lambda Role Credentials
IAM best practice. Handled automatically by AWS (no action needed for assumed roles).

---

## Troubleshooting

See [FAILURE_RUNBOOK.md](FAILURE_RUNBOOK.md) for detailed diagnostics.

**Quick checklist:**
1. [ ] CloudWatch logs show all 4 steps completed?
2. [ ] AWSCURRENT version has recent createdAt?
3. [ ] Public key visible in CloudFront KeyGroup?
4. [ ] No Lambda errors in last 24h?

---

## Upgrade Path

### When to Upgrade
- New go-infra-adapters version (security fixes)
- New Go runtime (security patches)
- Bug fix in rotation logic

### How to Upgrade
1. Update `go.mod` (if SDK version bumps):
   ```bash
   go get -u github.com/brunojet/go-infra-adapters/v4
   go mod tidy
   ```

2. Rebuild Lambda:
   ```bash
   ./scripts/build-lambda.sh
   ```

3. Deploy:
   ```bash
   cd terraform/env/dev
   terraform apply -var-file=terraform.tfvars
   ```

4. Verify: Manual rotation test (see above).

---

## Cost Optimization

**Current costs (monthly):**
- Lambda: $0.20 (once daily, ~100ms)
- Secrets Manager: $0.40 (secret storage)
- CloudFront: $1-2 (API calls for key ops)
- CloudWatch Logs: $0.50 (14 days retention)
- **Total: ~$2.10/month**

**To reduce:**
- Lower log retention (5 days instead of 14): saves $0.25
- Increase rotation interval (every 14d instead of 7d): saves $0.10 (half as many Lambda invokes)

---

## Disaster Recovery

### Backup/Restore Secret
```bash
# Export secret
aws secretsmanager get-secret-value \
  --secret-id /go-edge-key-management/rotator \
  --query SecretString \
  --output text > backup.json

# Restore (if secret deleted)
aws secretsmanager create-secret \
  --name /go-edge-key-management/rotator \
  --secret-string file://backup.json
```

### Recreate KeyGroup
If KeyGroup deleted accidentally:
```bash
# Manual trigger will recreate it
aws secretsmanager rotate-secret \
  --secret-id /go-edge-key-management/rotator \
  --rotate-immediately
```

Lambda createSecret step will detect KeyGroup missing → create it.

---

## On-Call Runbook

**Alert:** Lambda rotation failed

**Step 1 (5min):**
```bash
aws logs tail /aws/lambda/key-rotator --since 10m
```
Look for error message → consult [FAILURE_RUNBOOK.md](FAILURE_RUNBOOK.md).

**Step 2 (10min):**
If error is transient → manual retry:
```bash
aws secretsmanager rotate-secret \
  --secret-id /go-edge-key-management/rotator
```

**Step 3 (20min):**
If still failing → follow recovery steps in FAILURE_RUNBOOK.md.

**Step 4 (escalate):**
Collect logs (see FAILURE_RUNBOOK.md) → open AWS Support case.

---

## Contacts & Resources

- **Code repo:** https://github.com/brunojet/go-edge-key-management
- **AWS SDK:** https://github.com/brunojet/go-infra-adapters (v4)
- **Architecture:** See [ARCHITECTURE.md](ARCHITECTURE.md)
- **Failure guide:** See [FAILURE_RUNBOOK.md](FAILURE_RUNBOOK.md)
- **Risk analysis:** See [RISK_ANALYSIS.md](RISK_ANALYSIS.md)
