package rotator

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	urlpkg "net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cfTypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// secret data structures stored in Secrets Manager
type SecretEntry struct {
	PrivatePEM  string    `json:"private_pem"`
	PublicPEM   string    `json:"public_pem"`
	PublicKeyId string    `json:"public_key_id,omitempty"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}

type SecretPayload struct {
	Current    *SecretEntry `json:"current"`
	Previous   *SecretEntry `json:"previous,omitempty"`
	KeyGroupId string       `json:"key_group_id,omitempty"`
}

// Rotate performs one rotation cycle using AWS SDK v2.
// - secretName: Secrets Manager secret name (or ARN)
// - keyGroupIDArg: optional existing KeyGroupId (can be empty)
// - namePrefix: prefix for naming CloudFront resources
func Rotate(ctx context.Context, secretName string, keyGroupIDArg string, namePrefix string) (*SecretPayload, error) {
	log.Printf("Starting rotation process - secretName: %s, keyGroupIDArg: %s, namePrefix: %s", secretName, keyGroupIDArg, namePrefix)

	// Load AWS config with client logging enabled for troubleshooting
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithClientLogMode(aws.LogRetries|aws.LogRequest|aws.LogResponse),
	)
	if err != nil {
		log.Printf("ERROR: Failed to load AWS config: %v", err)
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	log.Printf("AWS Config loaded - Default region: %s", cfg.Region)

	// Test basic AWS connectivity first
	if err := testAWSConnectivity(ctx, cfg); err != nil {
		log.Printf("ERROR: AWS connectivity test failed: %v", err)
		return nil, fmt.Errorf("aws connectivity test failed: %w", err)
	}

	// Create Secrets Manager client
	sm := secretsmanager.NewFromConfig(cfg)
	log.Printf("SecretsManager client created for region: %s", cfg.Region)

	// Ensure secret resource exists — do NOT create it from the Lambda.
	// The infra (Terraform) must create the secret resource; Lambda only writes values to existing secrets.
	if _, derr := sm.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{SecretId: aws.String(secretName)}); derr != nil {
		if strings.Contains(derr.Error(), "ResourceNotFoundException") || strings.Contains(strings.ToLower(derr.Error()), "not found") {
			log.Printf("Secret resource %s not found. Aborting: this Lambda expects the secret resource to be created by infrastructure.", secretName)
			return nil, fmt.Errorf("secret resource %s not found; create it via Terraform", secretName)
		}

		log.Printf("DescribeSecret preliminary check failed: %v", derr)
		return nil, fmt.Errorf("describe secret preliminary: %w", derr)
	} else {
		log.Printf("Secret resource %s exists (pre-check)", secretName)
	}

	// Create CloudFront client with explicit us-east-1 region
	cfCfg := cfg
	cfCfg.Region = "us-east-1"
	cf := cloudfront.NewFromConfig(cfCfg)
	log.Printf("CloudFront client created for region: %s", cfCfg.Region)

	// Debug endpoint resolution
	debugEndpointResolution(cfg, cfCfg)

	// Network-level diagnostics: DNS resolution, TCP connect and TLS handshake
	debugNetworkConnectivity(ctx, cfg, cfCfg)

	// Test CloudFront connectivity specifically
	if err := testCloudFrontConnectivity(ctx, cf); err != nil {
		log.Printf("ERROR: CloudFront connectivity test failed: %v", err)
		return nil, fmt.Errorf("cloudfront connectivity test failed: %w", err)
	}

	// Try to read existing secret value; if the secret resource exists but has no value
	// (no AWSCURRENT) we'll create the initial secret value via PutSecretValue later.
	var existing SecretPayload
	secretHasValue := false
	secretResourceExists := false

	log.Printf("Attempting to retrieve secret value for: %s", secretName)
	got, err := sm.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(secretName)})
	if err != nil {
		log.Printf("GetSecretValue error: %v", err)
		if strings.Contains(err.Error(), "ResourceNotFoundException") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			// The secret resource may still exist but simply have no value/version yet.
			// Check DescribeSecret to confirm resource existence.
			if _, derr := sm.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{SecretId: aws.String(secretName)}); derr == nil {
				secretResourceExists = true
				secretHasValue = false
				log.Printf("Secret %s exists (no value). Will create initial secret value via PutSecretValue.", secretName)
			} else {
				log.Printf("Secret %s not found (DescribeSecret error: %v). Aborting.", secretName, derr)
				return nil, fmt.Errorf("secret %s not found; create it via Terraform or grant the Lambda permission to create secrets: %w", secretName, derr)
			}
		} else {
			return nil, fmt.Errorf("get secret value: %w", err)
		}
	} else {
		// Secret value retrieved successfully
		secretResourceExists = true
		if got.SecretString != nil {
			secretHasValue = true
			log.Printf("Secret retrieved successfully - VersionId=%s, SecretString present (length=%d)", aws.ToString(got.VersionId), len(*got.SecretString))
			if err := json.Unmarshal([]byte(*got.SecretString), &existing); err != nil {
				log.Printf("ERROR: Failed to unmarshal secret payload: %v", err)
				return nil, fmt.Errorf("unmarshal secret payload: %w", err)
			}
			log.Printf("Secret payload unmarshaled successfully (sensitive fields redacted)")
		} else {
			secretHasValue = false
			log.Printf("Secret retrieved successfully - VersionId=%s, no SecretString present", aws.ToString(got.VersionId))
		}
	}

	// Generate new key pair
	log.Printf("Generating new RSA key pair...")
	kp, err := GenerateRSAKeyPair(2048)
	if err != nil {
		log.Printf("ERROR: Failed to generate key pair: %v", err)
		return nil, fmt.Errorf("generate key pair: %w", err)
	}
	log.Printf("Key pair generated successfully - Fingerprint: %s", kp.Fingerprint[:8])

	// Create CloudFront public key with retry logic
	callerRef := fmt.Sprintf("%s-%d", namePrefix, time.Now().UnixNano())
	pubName := fmt.Sprintf("%s-%s", namePrefix, kp.Fingerprint[:8])
	log.Printf("Creating CloudFront public key - CallerRef: %s, Name: %s", callerRef, pubName)

	createInput := &cloudfront.CreatePublicKeyInput{
		PublicKeyConfig: &cfTypes.PublicKeyConfig{
			CallerReference: aws.String(callerRef),
			Name:            aws.String(pubName),
			EncodedKey:      aws.String(kp.PublicPEM),
		},
	}

	createOut, err := createCloudFrontPublicKeyWithRetry(ctx, cf, createInput)
	if err != nil {
		log.Printf("ERROR: Failed to create CloudFront public key: %v", err)
		return nil, fmt.Errorf("create cloudfront public key: %w", err)
	}
	newPublicKeyId := "<unknown>"
	if createOut != nil && createOut.PublicKey != nil {
		newPublicKeyId = aws.ToString(createOut.PublicKey.Id)
	}
	log.Printf("CloudFront public key created successfully - ID: %s, ETag=%s", newPublicKeyId, aws.ToString(createOut.ETag))
	log.Printf("CreatePublicKey full output (truncated): %+v", struct{ ID, ETag string }{ID: newPublicKeyId, ETag: aws.ToString(createOut.ETag)})

	// Continue with KeyGroup logic
	keyGroupId := keyGroupIDArg
	if keyGroupId == "" && existing.KeyGroupId != "" {
		keyGroupId = existing.KeyGroupId
	}

	if keyGroupId == "" {
		log.Printf("Creating new KeyGroup...")
		keyGroupId, err = createNewKeyGroup(ctx, cf, namePrefix, newPublicKeyId)
		if err != nil {
			return nil, err
		}
	} else {
		log.Printf("Updating existing KeyGroup: %s", keyGroupId)
		err = updateExistingKeyGroup(ctx, cf, keyGroupId, newPublicKeyId, &existing)
		if err != nil {
			return nil, err
		}
	}

	// Prepare and save secret payload
	outPayload := prepareSecretPayload(existing, kp, newPublicKeyId, keyGroupId, secretHasValue)

	if err := saveSecretPayload(ctx, sm, secretName, outPayload, secretResourceExists); err != nil {
		return nil, err
	}

	log.Printf("Rotation completed successfully - KeyGroupId: %s, PublicKeyId: %s", keyGroupId, newPublicKeyId)
	return outPayload, nil
}

// testAWSConnectivity performs basic AWS connectivity test
func testAWSConnectivity(ctx context.Context, cfg aws.Config) error {
	log.Printf("Testing basic AWS connectivity...")

	// Test with STS GetCallerIdentity (lightweight operation)
	stsClient := sts.NewFromConfig(cfg)
	_, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Printf("STS GetCallerIdentity failed: %v", err)
		return fmt.Errorf("sts connectivity test failed: %w", err)
	}

	log.Printf("Basic AWS connectivity test passed")
	return nil
}

// testCloudFrontConnectivity tests CloudFront API connectivity
func testCloudFrontConnectivity(ctx context.Context, cf *cloudfront.Client) error {
	log.Printf("Testing CloudFront API connectivity...")

	// Use ListPublicKeys as a lightweight test operation
	listInput := &cloudfront.ListPublicKeysInput{
		MaxItems: aws.Int32(1),
	}

	listOut, err := cf.ListPublicKeys(ctx, listInput)
	if err != nil {
		log.Printf("CloudFront ListPublicKeys test failed: %v", err)

		// Check for specific error types
		if strings.Contains(err.Error(), "ResolveEndpointV2") {
			log.Printf("DIAGNOSIS: ResolveEndpointV2 error indicates endpoint resolution failure")
			log.Printf("POSSIBLE CAUSES:")
			log.Printf("1. Network connectivity issues (VPC/NAT Gateway)")
			log.Printf("2. DNS resolution problems")
			log.Printf("3. AWS SDK configuration issues")
			log.Printf("4. Regional endpoint availability")
		}

		if strings.Contains(err.Error(), "not found") {
			log.Printf("DIAGNOSIS: 'not found' error suggests DNS or endpoint resolution failure")
			log.Printf("POSSIBLE CAUSES:")
			log.Printf("1. Lambda is in VPC without proper internet access")
			log.Printf("2. DNS resolution failing for CloudFront endpoints")
			log.Printf("3. Firewall/Security Group blocking HTTPS traffic")
		}

		return fmt.Errorf("cloudfront connectivity test failed: %w", err)
	} else {
		n := 0
		if listOut.PublicKeyList != nil {
			n = len(listOut.PublicKeyList.Items)
		}
		log.Printf("CloudFront ListPublicKeys returned %d items (truncated)", n)
		log.Printf("CloudFront API connectivity test passed")
	}
	return nil
}

// debugEndpointResolution logs endpoint resolution details
func debugEndpointResolution(cfg, cfCfg aws.Config) {
	log.Printf("=== ENDPOINT RESOLUTION DEBUG ===")
	log.Printf("Default region: %s", cfg.Region)
	log.Printf("CloudFront region: %s", cfCfg.Region)

	// Try to resolve endpoints manually for debugging
	if resolver, ok := cfg.EndpointResolver.(aws.EndpointResolverWithOptions); ok {
		// Test SecretsManager endpoint
		if ep, err := resolver.ResolveEndpoint("secretsmanager", cfg.Region); err == nil {
			log.Printf("SecretsManager endpoint resolved: %s (signing region: %s)", ep.URL, ep.SigningRegion)
		} else {
			log.Printf("SecretsManager endpoint resolution failed: %v", err)
		}

		// Test CloudFront endpoint
		if ep, err := resolver.ResolveEndpoint("cloudfront", cfCfg.Region); err == nil {
			log.Printf("CloudFront endpoint resolved: %s (signing region: %s)", ep.URL, ep.SigningRegion)
		} else {
			log.Printf("CloudFront endpoint resolution failed: %v", err)
		}
	} else {
		log.Printf("EndpointResolver not available for debugging")
	}
	log.Printf("=== END ENDPOINT RESOLUTION DEBUG ===")
}

// debugNetworkConnectivity performs DNS lookup, TCP connect and TLS handshake
// to the service endpoints resolved by the SDK. Useful to diagnose ResolveEndpointV2
// and network/VPC/NAT/DNS issues from Lambda.
func debugNetworkConnectivity(ctx context.Context, cfg, cfCfg aws.Config) {
	services := []struct {
		name   string
		region string
	}{
		{"cloudfront", cfCfg.Region},
		{"secretsmanager", cfg.Region},
		{"sts", cfg.Region},
	}

	var resolver aws.EndpointResolverWithOptions
	if r, ok := cfg.EndpointResolver.(aws.EndpointResolverWithOptions); ok {
		resolver = r
	}

	for _, s := range services {
		log.Printf("--- Network diagnostics for service=%s region=%s ---", s.name, s.region)

		var ep aws.Endpoint
		if resolver != nil {
			e, err := resolver.ResolveEndpoint(s.name, s.region)
			if err != nil {
				log.Printf("ResolveEndpoint(%s,%s) error: %v", s.name, s.region, err)
			} else {
				ep = e
				log.Printf("Resolved endpoint for %s: URL=%s SigningRegion=%s", s.name, ep.URL, ep.SigningRegion)
			}
		} else {
			log.Printf("No EndpointResolver available in cfg for service %s", s.name)
		}

		if ep.URL == "" {
			log.Printf("No resolved URL for %s — skipping DNS/TCP checks", s.name)
			continue
		}

		u, err := urlpkg.Parse(ep.URL)
		if err != nil {
			log.Printf("Failed to parse endpoint URL %s: %v", ep.URL, err)
			continue
		}
		host := u.Hostname()
		if host == "" {
			host = u.Host
		}

		// DNS lookup
		ips, err := net.LookupIP(host)
		if err != nil {
			log.Printf("DNS lookup failed for host %s: %v", host, err)
		} else {
			lim := len(ips)
			if lim > 5 {
				lim = 5
			}
			ipstr := make([]string, 0, lim)
			for i := 0; i < lim; i++ {
				ipstr = append(ipstr, ips[i].String())
			}
			log.Printf("DNS lookup for %s -> %v (showing up to 5)", host, ipstr)
		}

		// TCP dial
		addr := net.JoinHostPort(host, "443")
		d := &net.Dialer{Timeout: 5 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			log.Printf("TCP dial to %s failed: %v", addr, err)
			// If DNS produced IPs, try direct connect to first IP
			if len(ips) > 0 {
				ipAddr := net.JoinHostPort(ips[0].String(), "443")
				log.Printf("Attempting TCP connect to IP %s", ipAddr)
				if conn2, err2 := d.DialContext(ctx, "tcp", ipAddr); err2 != nil {
					log.Printf("TCP connect to IP %s failed: %v", ipAddr, err2)
				} else {
					log.Printf("TCP connect to IP %s succeeded", ipAddr)
					conn2.Close()
				}
			}
		} else {
			log.Printf("TCP dial to %s succeeded", addr)
			conn.Close()

			// TLS handshake (InsecureSkipVerify=true to surface handshake errors)
			tlsCfg := &tls.Config{ServerName: host, InsecureSkipVerify: true}
			if tlsConn, err := tls.DialWithDialer(d, "tcp", addr, tlsCfg); err != nil {
				log.Printf("TLS handshake to %s failed: %v", addr, err)
			} else {
				state := tlsConn.ConnectionState()
				if len(state.PeerCertificates) > 0 {
					cert := state.PeerCertificates[0]
					log.Printf("TLS cert for %s: Subject=%s Issuer=%s NotAfter=%s", host, cert.Subject.CommonName, cert.Issuer.CommonName, cert.NotAfter)
				} else {
					log.Printf("TLS handshake succeeded but no peer certificates returned for %s", host)
				}
				tlsConn.Close()
			}
		}
	}
}

// createCloudFrontPublicKeyWithRetry attempts to create public key with retry logic
func createCloudFrontPublicKeyWithRetry(ctx context.Context, cf *cloudfront.Client, input *cloudfront.CreatePublicKeyInput) (*cloudfront.CreatePublicKeyOutput, error) {
	maxRetries := 3
	baseDelay := time.Second * 2

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("Attempting to create CloudFront public key (attempt %d/%d)", attempt, maxRetries)

		createOut, err := cf.CreatePublicKey(ctx, input)
		if err == nil {
			return createOut, nil
		}

		log.Printf("Attempt %d failed: %v", attempt, err)

		// Check for specific error patterns
		if strings.Contains(err.Error(), "ResolveEndpointV2") {
			log.Printf("ResolveEndpointV2 error detected on attempt %d", attempt)
		}

		if strings.Contains(err.Error(), "not found") {
			log.Printf("Endpoint 'not found' error detected on attempt %d", attempt)
		}

		// Don't retry on the last attempt
		if attempt < maxRetries {
			delay := time.Duration(attempt) * baseDelay
			log.Printf("Retrying in %v...", delay)
			time.Sleep(delay)
		}
	}

	return nil, fmt.Errorf("failed to create public key after %d attempts", maxRetries)
}

// createNewKeyGroup creates a new CloudFront KeyGroup
func createNewKeyGroup(ctx context.Context, cf *cloudfront.Client, namePrefix, publicKeyId string) (string, error) {
	kgName := fmt.Sprintf("%s-kg", namePrefix)
	kgInput := &cloudfront.CreateKeyGroupInput{
		KeyGroupConfig: &cfTypes.KeyGroupConfig{
			Name:  aws.String(kgName),
			Items: []string{publicKeyId},
		},
	}

	kgOut, err := cf.CreateKeyGroup(ctx, kgInput)
	if err != nil {
		log.Printf("ERROR: Failed to create KeyGroup: %v", err)
		return "", fmt.Errorf("create key group: %w", err)
	}

	keyGroupId := aws.ToString(kgOut.KeyGroup.Id)
	log.Printf("KeyGroup created successfully - ID: %s, ETag=%s", keyGroupId, aws.ToString(kgOut.ETag))
	return keyGroupId, nil
}

// updateExistingKeyGroup updates an existing CloudFront KeyGroup
func updateExistingKeyGroup(ctx context.Context, cf *cloudfront.Client, keyGroupId, newPublicKeyId string, existing *SecretPayload) error {
	getInput := &cloudfront.GetKeyGroupInput{Id: aws.String(keyGroupId)}
	getKgOut, err := cf.GetKeyGroup(ctx, getInput)
	if err != nil {
		log.Printf("ERROR: Failed to get KeyGroup: %v", err)
		return fmt.Errorf("get key group: %w", err)
	}

	var existingItems []string
	if getKgOut.KeyGroup != nil && getKgOut.KeyGroup.KeyGroupConfig != nil {
		existingItems = getKgOut.KeyGroup.KeyGroupConfig.Items
	}

	newItems := []string{newPublicKeyId}
	if len(existingItems) > 0 {
		if existing.Current != nil && existing.Current.PublicKeyId != "" {
			if existing.Current.PublicKeyId != newPublicKeyId {
				newItems = append(newItems, existing.Current.PublicKeyId)
			}
		} else if len(existingItems) > 0 {
			if existingItems[0] != newPublicKeyId {
				newItems = append(newItems, existingItems[0])
			}
		}
	}

	kgConfig := cfTypes.KeyGroupConfig{
		Name:  getKgOut.KeyGroup.KeyGroupConfig.Name,
		Items: newItems,
	}

	updateInput := &cloudfront.UpdateKeyGroupInput{
		Id:             aws.String(keyGroupId),
		IfMatch:        getKgOut.ETag,
		KeyGroupConfig: &kgConfig,
	}

	updateOut, err := cf.UpdateKeyGroup(ctx, updateInput)
	if err != nil {
		log.Printf("ERROR: Failed to update KeyGroup: %v", err)
		return fmt.Errorf("update key group: %w", err)
	}

	log.Printf("KeyGroup updated successfully - ETag=%s", aws.ToString(updateOut.ETag))
	return nil
}

// prepareSecretPayload prepares the secret payload for storage
func prepareSecretPayload(existing SecretPayload, kp *KeyPair, newPublicKeyId, keyGroupId string, secretExists bool) *SecretPayload {
	var outPayload SecretPayload
	if secretExists {
		outPayload = existing
	}

	if outPayload.Current != nil {
		outPayload.Previous = outPayload.Current
	}

	outPayload.Current = &SecretEntry{
		PrivatePEM:  kp.PrivatePEM,
		PublicPEM:   kp.PublicPEM,
		PublicKeyId: newPublicKeyId,
		Fingerprint: kp.Fingerprint,
		CreatedAt:   kp.CreatedAt,
	}
	outPayload.KeyGroupId = keyGroupId

	return &outPayload
}

// saveSecretPayload saves the secret payload to Secrets Manager. It will
// only write values to an existing secret resource; it will not create
// secret resources. Creation of the secret must be managed by infrastructure.
func saveSecretPayload(ctx context.Context, sm *secretsmanager.Client, secretName string, payload *SecretPayload, resourceExists bool) error {
	b, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ERROR: Failed to marshal secret payload: %v", err)
		return fmt.Errorf("marshal secret payload: %w", err)
	}

	if resourceExists {
		log.Printf("Putting secret value (update or initial value for existing secret resource)...")
		putOut, err := sm.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
			SecretId:     aws.String(secretName),
			SecretString: aws.String(string(b)),
		})
		if err != nil {
			log.Printf("ERROR: Failed to put secret value: %v", err)
			return fmt.Errorf("put secret value: %w", err)
		}
		log.Printf("PutSecretValue succeeded - VersionId=%s", aws.ToString(putOut.VersionId))
	}

	if !resourceExists {
		log.Printf("ERROR: Secret resource %s does not exist; refusing to create it from Lambda", secretName)
		return fmt.Errorf("secret resource %s does not exist; creation is managed by infrastructure", secretName)
	}

	log.Printf("Secret saved successfully (metadata logged, sensitive contents redacted)")
	return nil
}
