# Architecture Diagrams: go-edge-key-management

Mermaid diagrams for architecture overview and rotation sequence.

---

## System Architecture

```mermaid
graph TB
    subgraph AWS["AWS Services"]
        SM["Secrets Manager<br/>(stores RSA pairs)"]
        Lambda["Lambda Function<br/>(rotation orchestrator)"]
        CF["CloudFront<br/>(key distribution)"]
        CW["CloudWatch<br/>(logs & metrics)"]
        IAM["IAM Role<br/>(permissions)"]
    end

    subgraph Client["Client"]
        Client1["CloudFront<br/>Distribution"]
    end

    SM -->|rotation event| Lambda
    Lambda -->|create/update versions| SM
    Lambda -->|create keys,<br/>manage key group| CF
    Lambda -->|structured logs| CW
    Lambda -->|use role| IAM
    CF -->|serve key group| Client1

    style SM fill:#ff9999
    style Lambda fill:#99ccff
    style CF fill:#99ff99
    style CW fill:#ffcc99
    style IAM fill:#ff99cc
```

**Components:**
- **Secrets Manager** — Stores current + previous RSA keypairs, triggers rotation
- **Lambda** — Handler for 4-step rotation contract (createSecret → setSecret → testSecret → finishSecret)
- **CloudFront** — Distributes public keys via key groups
- **CloudWatch** — Structured logging (slog), metrics, alarms
- **IAM** — Least-privilege role for Lambda access

---

## 4-Step Rotation Sequence

```mermaid
sequenceDiagram
    participant SM as Secrets Manager
    participant Lambda as Lambda Handler
    participant CF as CloudFront API
    participant CW as CloudWatch

    Note over SM,CW: STEP 1: createSecret

    SM->>Lambda: RotationEvent {step: "createSecret", token: "T1"}
    Lambda->>Lambda: Check MIN_ROTATION_INTERVAL (60min)
    Lambda->>Lambda: Generate RSA 2048 keypair
    Lambda->>CF: CreatePublicKey(PEM)<br/>→ publicKeyID
    alt Public Key Creation Fails
        Lambda->>Lambda: Cleanup: remove orphan key
        Lambda->>CW: ERROR: "create public key"
        Lambda-->>SM: Error (retry)
    else Success
        Lambda->>SM: PutSecretValue(privateKey,<br/>publicKey, publicKeyID)<br/>version: AWSPENDING, token: T1
        Lambda->>CW: INFO: createSecret complete
    end

    Note over SM,CW: STEP 2: setSecret

    SM->>Lambda: RotationEvent {step: "setSecret", token: "T1"}
    Lambda->>SM: GetSecretValue(AWSPENDING, T1)
    Lambda->>CF: EnsureKeyGroup(keyGroupName,<br/>publicKeyID)
    alt KeyGroup Update Fails
        Lambda->>CF: DeletePublicKey(publicKeyID)
        Lambda->>CW: ERROR: "ensure key group"
        Lambda-->>SM: Error (retry)
    else Success
        Lambda->>CW: INFO: setSecret complete
    end

    Note over SM,CW: STEP 3: testSecret

    SM->>Lambda: RotationEvent {step: "testSecret", token: "T1"}
    Lambda->>SM: GetSecretValue(AWSPENDING, T1)
    Lambda->>CF: VerifyKeyInGroup(keyGroupName,<br/>publicKeyID)
    alt Key Not Visible
        Lambda->>CF: DeletePublicKey(publicKeyID)
        Lambda->>CW: ERROR: "verify pending key"
        Lambda-->>SM: Error (retry)
    else Key Verified
        Lambda->>CW: INFO: testSecret complete
    end

    Note over SM,CW: STEP 4: finishSecret

    SM->>Lambda: RotationEvent {step: "finishSecret", token: "T1"}
    Lambda->>SM: PromoteVersion(token: T1)<br/>AWSPENDING → AWSCURRENT
    alt Promotion Fails
        Lambda->>CW: ERROR: "finish secret"
        Lambda-->>SM: Error (retry)
    else Success
        SM->>SM: Auto: AWSCURRENT → AWSPREVIOUS
        Lambda->>CW: INFO: finishSecret complete
        Note over SM: Rotation Complete ✅
    end
```

**Key Points:**
1. **createSecret** — Generate keypair, create public key in CloudFront
   - Idempotent: cleans up orphan keys on failure
   - Enforces MinRotationInterval (60min default)

2. **setSecret** — Add public key to CloudFront KeyGroup
   - Fails if KeyGroup update fails (with cleanup)
   - Evicts oldest key if at MaxKeysInGroup limit

3. **testSecret** — Verify public key is visible in KeyGroup
   - Gate: prevents timing gaps between create and visibility
   - Fails if key not yet propagated

4. **finishSecret** — Promote AWSPENDING to AWSCURRENT
   - Idempotent: same token = safe retry
   - Secrets Manager auto-handles AWSCURRENT → AWSPREVIOUS

---

## Cleanup Flow (Error Path)

```mermaid
graph TD
    A["Step Fails"]
    A -->|setSecret fails| B["DeletePublicKey<br/>(sanitize orphan)"]
    A -->|testSecret fails| C["DeletePublicKey<br/>(sanitize orphan)"]
    A -->|finishSecret fails| D["Log Error<br/>(no cleanup needed)"]
    
    B --> E["Abort Rotation<br/>(return error)"]
    C --> E
    D --> E
    
    E --> F["Secrets Manager Retries<br/>(up to 10 times)"]
    F -->|Next Attempt| G["Same 4 Steps"]
    
    style A fill:#ff6666
    style B fill:#ffcc99
    style C fill:#ffcc99
    style D fill:#ffcc99
    style E fill:#ff9999
    style F fill:#ffff99
    style G fill:#99ff99
```

**Cleanup Strategy:**
- **createSecret fail:** Remove orphan public key before return
- **setSecret fail:** Remove orphan public key before abort
- **testSecret fail:** Remove orphan public key before abort
- **finishSecret fail:** No cleanup needed (already in CloudFront KeyGroup)

Result: No orphan resources accumulate on failure.

---

## State Machine: Secrets Manager Versions

```mermaid
stateDiagram-v2
    [*] --> AWSCURRENT
    
    AWSCURRENT --> AWSPENDING: Rotation Triggered
    
    AWSPENDING --> AWSPENDING: createSecret (idempotent)
    AWSPENDING --> AWSPENDING: setSecret (idempotent)
    AWSPENDING --> AWSPENDING: testSecret (idempotent)
    
    AWSPENDING --> AWSCURRENT: finishSecret succeeds
    AWSPENDING --> [*]: finishSecret → cleanup + promote
    
    AWSCURRENT --> AWSPREVIOUS: Auto-transition<br/>(after promotion)
    AWSPREVIOUS --> [*]: Auto-deleted<br/>(after next rotation)
    
    note right of AWSPENDING
        Stuck here if Lambda dies
        MinRotationInterval prevents loop
        Manual rotation overrides
    end note
    
    note right of AWSCURRENT
        Active version
        In use by clients
    end note
    
    note right of AWSPREVIOUS
        Previous version
        Kept for backward compat
        Auto-deleted on next rotation
    end note
```

**Version Lifecycle:**
1. Start: `AWSCURRENT` only
2. Rotation triggered: Create `AWSPENDING`
3. All 4 steps complete: `AWSPENDING` → `AWSCURRENT`
4. Auto: Old `AWSCURRENT` → `AWSPREVIOUS`
5. Next rotation: `AWSPREVIOUS` auto-deleted

---

## CloudFront KeyGroup Lifecycle

```mermaid
graph LR
    A["KeyGroup Empty"]
    B["KeyGroup: [Key1]"]
    C["KeyGroup: [Key2, Key1]"]
    D["KeyGroup: [Key3, Key2]"]
    E["KeyGroup: [Key1, Key3]"]
    
    A -->|createSecret<br/>add Key1| B
    B -->|createSecret<br/>add Key2| C
    C -->|createSecret<br/>evict Key1<br/>add Key3| D
    D -->|createSecret<br/>evict Key2<br/>add Key1| E
    
    style A fill:#cccccc
    style B fill:#99ff99
    style C fill:#99ff99
    style D fill:#99ff99
    style E fill:#99ff99
    
    Note over B,E: MAX_KEYS_IN_GROUP: 3<br/>Oldest key evicted when at limit
```

**Key Group Management:**
- Max size: `MaxKeysInGroup` (default 3)
- Order: Newest first (insertion order)
- Eviction: Oldest removed when at limit
- Idempotent: Adding same key twice is no-op

---

## Monitoring & Alerting Flow

```mermaid
graph TB
    Lambda["Lambda<br/>Execution"]
    Logs["CloudWatch Logs<br/>(slog JSON)"]
    Metrics["CloudWatch Metrics<br/>(Duration, Errors, etc)"]
    
    Alarms["CloudWatch Alarms"]
    SNS["SNS Notification"]
    OnCall["On-Call<br/>Operator"]
    
    Lambda -->|stdout| Logs
    Logs -->|parsed| Metrics
    
    Metrics -->|Duration > 30s| Alarms
    Metrics -->|Errors > 0| Alarms
    Metrics -->|Throttles| Alarms
    
    Alarms -->|trigger| SNS
    SNS -->|notify| OnCall
    
    OnCall -->|read logs| Logs
    OnCall -->|check state| Lambda
    
    style Lambda fill:#99ccff
    style Logs fill:#ffcc99
    style Metrics fill:#ffcc99
    style Alarms fill:#ff9999
    style SNS fill:#ff9999
    style OnCall fill:#ff6666
```

**Observability:**
- **Logs** — Structured JSON per step (slog)
- **Metrics** — Duration, errors, throttles, concurrency
- **Alarms** — Errors > 0, Duration > 30s, Throttles > 0
- **On-Call** — Notified via SNS, uses FAILURE_RUNBOOK.md to diagnose

---

## Data Flow: Secret Payload

```mermaid
graph LR
    Gen["Generate<br/>RSA 2048"]
    Payload["SecretPayload<br/>{<br/>  privateKey,<br/>  publicKey,<br/>  fingerprint,<br/>  publicKeyID,<br/>  createdAt<br/>}"]
    SM["Secrets Manager<br/>(AWSPENDING)"]
    CF["CloudFront<br/>PublicKey"]
    Client["Client<br/>(uses key)"]
    
    Gen --> Payload
    Payload --> SM
    Payload --> CF
    CF --> Client
    
    style Gen fill:#99ff99
    style Payload fill:#ffff99
    style SM fill:#ff9999
    style CF fill:#99ff99
    style Client fill:#cccccc
```

**Payload Contents:**
- `privateKey` — PEM-encoded, stored in Secrets Manager
- `publicKey` — PEM-encoded, sent to CloudFront
- `fingerprint` — SHA256 hash, unique per keypair
- `publicKeyID` — AWS-assigned CloudFront ID
- `createdAt` — Timestamp for audit

---

## Error Recovery Loop

```mermaid
graph TD
    A["Rotation Triggered"]
    B["Execute Step"]
    C{Step<br/>Succeeds?}
    
    D["Next Step"]
    E{All 4<br/>Steps Done?}
    
    F["Cleanup<br/>(remove orphan keys)"]
    G["Log Error"]
    H["Return Error"]
    
    I["Rotation Complete ✅"]
    
    A --> B
    B --> C
    
    C -->|Yes| D
    D --> E
    
    E -->|No| B
    E -->|Yes| I
    
    C -->|No| F
    F --> G
    G --> H
    
    H -.->|Secrets Manager<br/>Retry Policy| B
    
    style A fill:#99ff99
    style B fill:#99ccff
    style C fill:#ffff99
    style D fill:#99ccff
    style E fill:#ffff99
    style F fill:#ffcc99
    style G fill:#ff9999
    style H fill:#ff6666
    style I fill:#99ff99
```

**Flow:**
1. Execute step (createSecret, setSecret, testSecret, finishSecret)
2. If fails → cleanup + log error + return to Secrets Manager
3. If succeeds → move to next step
4. Secrets Manager retries (automatic, up to 10 times)
5. Success → rotation complete

---

## References

- Full details: [ARCHITECTURE.md](ARCHITECTURE.md)
- Troubleshooting: [FAILURE_RUNBOOK.md](FAILURE_RUNBOOK.md)
- Operations: [OPERATIONS.md](OPERATIONS.md)
- Risk analysis: [RISK_ANALYSIS.md](RISK_ANALYSIS.md)
