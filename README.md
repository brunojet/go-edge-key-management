# go-edge-key-management

Lambda-based RSA key rotation for CloudFront Signed URLs.

---

## What This Does

Automatically rotates RSA keypairs used to sign CloudFront URLs. 

**Workflow:**
1. Secrets Manager triggers Lambda daily/weekly (configurable)
2. Lambda generates new RSA keypair
3. Lambda updates CloudFront key group
4. Secrets Manager stores new keypair
5. Repeat

Result: **Zero-downtime key rotation, fully automated.**

---

## Quick Links

| Document | Purpose |
|----------|---------|
| [**ARCHITECTURE.md**](docs/ARCHITECTURE.md) | Design, flow, components, decisions |
| [**OPERATIONS.md**](docs/OPERATIONS.md) | Monitoring, maintenance, procedures |
| [**FAILURE_RUNBOOK.md**](docs/FAILURE_RUNBOOK.md) | Troubleshooting & recovery |
| [**RISK_ANALYSIS.md**](docs/RISK_ANALYSIS.md) | Security & operational risks |

---

## Local Development

### Prerequisites
- Go 1.26.3+
- AWS credentials (for integration tests; local tests work offline)

### Run Tests
```bash
go test ./...          # All tests
go test -v ./...       # Verbose
go test -cover ./...   # With coverage
```

### Local Simulation
```bash
go run ./cmd/rotator -out ./tmp-output
```
Generates a test secret JSON in `tmp-output/` (no AWS calls).

### Code Organization
```
cmd/rotator/          # Lambda entry point
internal/
  ├── handler/        # AWS client wiring
  ├── rotator/        # 4-step rotation logic
  └── domain/         # Data types
terraform/
  ├── modules/        # key_rotator module
  └── env/dev/        # Development config
scripts/
  └── build-lambda.sh # Build & zip for Lambda
docs/                 # Documentation (see above)
```

---

## Deployment

### Build
```bash
./scripts/build-lambda.sh
# Creates: rotator.zip
```

### Provision (Terraform)
```bash
cd terraform/env/dev
terraform init
terraform apply -var-file=terraform.tfvars
```

### Verify
Check CloudWatch logs:
```bash
aws logs tail /aws/lambda/key-rotator --follow
```

See [OPERATIONS.md](docs/OPERATIONS.md) for manual testing & monitoring.

---

## Configuration

Environment variables (set by Terraform):

| Variable | Default | Purpose |
|----------|---------|---------|
| `SECRET_NAME` | (required) | Secrets Manager secret |
| `KEY_GROUP_NAME` | (required) | CloudFront key group |
| `MIN_ROTATION_INTERVAL_MINUTES` | 60 | Min time between rotations |
| `MAX_KEYS_IN_GROUP` | 3 | Keys to keep in CloudFront |
| `CLOUDFRONT_CONCURRENCY` | 5 | Parallel API calls |

See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for design rationale.

---

## Troubleshooting

**Rotation failed?**
→ See [FAILURE_RUNBOOK.md](docs/FAILURE_RUNBOOK.md)

**Want to understand the design?**
→ See [ARCHITECTURE.md](docs/ARCHITECTURE.md)

**Concerned about security/risks?**
→ See [RISK_ANALYSIS.md](docs/RISK_ANALYSIS.md)

**How to monitor/maintain?**
→ See [OPERATIONS.md](docs/OPERATIONS.md)

---

## Key Facts

- **Language:** Go 1.26.3
- **Runtime:** Lambda (ARM64, custom Go runtime)
- **Schedule:** Secrets Manager rotation (daily/weekly, configurable)
- **Key type:** RSA 2048-bit
- **State:** Secrets Manager (versions) + CloudFront (key group)
- **Cost:** ~$2/month (Lambda + APIs + Secrets Manager)
- **SLA:** Automated daily rotation, no manual intervention

---

## Minimal Viable Checklist

Before going to production:

- [ ] Read [ARCHITECTURE.md](docs/ARCHITECTURE.md)
- [ ] Run tests: `go test ./...`
- [ ] Build: `./scripts/build-lambda.sh`
- [ ] Deploy: `terraform apply`
- [ ] Manual test: Trigger rotation, check CloudWatch logs
- [ ] Set up monitoring: See [OPERATIONS.md](docs/OPERATIONS.md)
- [ ] Read [FAILURE_RUNBOOK.md](docs/FAILURE_RUNBOOK.md) (in case needed)

---

## Support

- **Issues/PRs:** https://github.com/brunojet/go-edge-key-management
- **Questions:** Open an issue or check [ARCHITECTURE.md](docs/ARCHITECTURE.md) FAQ
- **Operational help:** See [OPERATIONS.md](docs/OPERATIONS.md) & [FAILURE_RUNBOOK.md](docs/FAILURE_RUNBOOK.md)

---

## License

[Add license here if applicable]
