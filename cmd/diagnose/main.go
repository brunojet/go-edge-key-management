// diagnose is a standalone CLI tool for debugging AWS connectivity issues.
// Run it locally or from a Lambda test invocation to surface DNS, TCP and
// TLS problems before deploying the rotator.
//
// Usage:
//
//	go run ./cmd/diagnose
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// serviceHost returns the default public hostname for each AWS service.
// No deprecated resolver needed — these follow the standard regional
// endpoint pattern documented by AWS.
func serviceHost(service, region string) string {
	switch service {
	case "cloudfront":
		// CloudFront is a global service; its API endpoint is always us-east-1.
		return "cloudfront.amazonaws.com"
	case "secretsmanager":
		return fmt.Sprintf("secretsmanager.%s.amazonaws.com", region)
	case "sts":
		return fmt.Sprintf("sts.%s.amazonaws.com", region)
	default:
		return fmt.Sprintf("%s.%s.amazonaws.com", service, region)
	}
}

func main() {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithClientLogMode(aws.LogRetries|aws.LogRequest|aws.LogResponse),
	)
	if err != nil {
		log.Fatalf("load aws config: %v", err)
	}

	log.Printf("AWS config loaded — region: %s", cfg.Region)

	cfCfg := cfg.Copy()
	cfCfg.Region = "us-east-1"

	checkIdentity(ctx, cfg)
	checkEndpoints(cfg, cfCfg)
	checkNetworkConnectivity(ctx, cfg, cfCfg)
	checkCloudFront(ctx, cfCfg)
	checkSecretsManager(ctx, cfg)
}

func checkIdentity(ctx context.Context, cfg aws.Config) {
	log.Println("=== STS: GetCallerIdentity ===")
	out, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Printf("FAIL: %v", err)
		return
	}
	log.Printf("OK — account: %s, arn: %s", aws.ToString(out.Account), aws.ToString(out.Arn))
}

func checkEndpoints(cfg, cfCfg aws.Config) {
	log.Println("=== Endpoint resolution ===")
	for _, svc := range []struct{ name, region string }{
		{"secretsmanager", cfg.Region},
		{"cloudfront", cfCfg.Region},
		{"sts", cfg.Region},
	} {
		host := serviceHost(svc.name, svc.region)
		log.Printf("%s → https://%s", svc.name, host)
	}
}

func checkNetworkConnectivity(ctx context.Context, cfg, cfCfg aws.Config) {
	log.Println("=== Network: DNS + TCP + TLS ===")

	services := []struct{ name, region string }{
		{"cloudfront", cfCfg.Region},
		{"secretsmanager", cfg.Region},
		{"sts", cfg.Region},
	}

	for _, s := range services {
		host := serviceHost(s.name, s.region)
		log.Printf("--- %s → %s ---", s.name, host)
		probeHost(ctx, host)
	}
}

func probeHost(ctx context.Context, host string) {
	// DNS
	ips, err := net.LookupIP(host)
	if err != nil {
		log.Printf("DNS %s: FAIL — %v", host, err)
		return
	}
	lim := len(ips)
	if lim > 5 {
		lim = 5
	}
	addrs := make([]string, lim)
	for i := range addrs {
		addrs[i] = ips[i].String()
	}
	log.Printf("DNS %s: %v", host, addrs)

	// TCP
	addr := net.JoinHostPort(host, "443")
	d := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		log.Printf("TCP %s: FAIL — %v", addr, err)
		return
	}
	conn.Close()
	log.Printf("TCP %s: OK", addr)

	// TLS
	tlsCfg := &tls.Config{ServerName: host, InsecureSkipVerify: true} //nolint:gosec // diagnostic tool
	tlsConn, err := tls.DialWithDialer(d, "tcp", addr, tlsCfg)
	if err != nil {
		log.Printf("TLS %s: FAIL — %v", addr, err)
		return
	}
	state := tlsConn.ConnectionState()
	tlsConn.Close()
	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		log.Printf("TLS %s: OK — CN=%s, expires: %s", host, cert.Subject.CommonName, cert.NotAfter.Format(time.DateOnly))
	} else {
		log.Printf("TLS %s: OK (no peer certs)", host)
	}
}

func checkCloudFront(ctx context.Context, cfCfg aws.Config) {
	log.Println("=== CloudFront: ListPublicKeys ===")
	cf := cloudfront.NewFromConfig(cfCfg)
	out, err := cf.ListPublicKeys(ctx, &cloudfront.ListPublicKeysInput{MaxItems: aws.Int32(1)})
	if err != nil {
		log.Printf("FAIL: %v", err)
		diagnoseEndpointError(err)
		return
	}
	n := 0
	if out.PublicKeyList != nil {
		n = len(out.PublicKeyList.Items)
	}
	log.Printf("OK — items returned: %d (max 1)", n)
}

func checkSecretsManager(ctx context.Context, cfg aws.Config) {
	secretName := os.Getenv("SECRET_NAME")
	if secretName == "" {
		log.Println("=== Secrets Manager: skipped (SECRET_NAME not set) ===")
		return
	}

	log.Printf("=== Secrets Manager: DescribeSecret %s ===", secretName)
	sm := secretsmanager.NewFromConfig(cfg)
	out, err := sm.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		log.Printf("FAIL: %v", err)
		return
	}
	log.Printf("OK — arn: %s", aws.ToString(out.ARN))
}

func diagnoseEndpointError(err error) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "ResolveEndpointV2"):
		fmt.Fprintln(os.Stderr, "DIAGNOSIS: endpoint resolution failure — check SDK version or VPC endpoint config")
	case strings.Contains(msg, "not found"):
		fmt.Fprintln(os.Stderr, "DIAGNOSIS: DNS or endpoint not found — check NAT Gateway / VPC DNS settings")
	case strings.Contains(msg, "timeout"):
		fmt.Fprintln(os.Stderr, "DIAGNOSIS: connection timed out — check Security Groups and NAT Gateway")
	}
}
