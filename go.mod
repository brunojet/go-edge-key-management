module github.com/brunojet/go-edge-key-management

go 1.25.0

require (
	github.com/aws/aws-lambda-go v1.54.0
	github.com/aws/aws-sdk-go-v2 v1.41.9
	github.com/aws/aws-sdk-go-v2/config v1.32.20
	github.com/aws/aws-sdk-go-v2/service/cloudfront v1.64.2
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.41.9
	github.com/aws/aws-sdk-go-v2/service/sts v1.42.3
	github.com/brunojet/go-infra-adapters/v3 v3.0.0-00010101000000-000000000000
)

replace github.com/brunojet/go-infra-adapters/v3 => ../go-infra-adapters

require (
	github.com/aws/aws-sdk-go-v2/credentials v1.19.19 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.25 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.25 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.25 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.26 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.25 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.1.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.36.2 // indirect
	github.com/aws/smithy-go v1.26.0 // indirect
	github.com/brunojet/go-infra-adapters v0.0.0-20260530101703-dccd05476031 // indirect
	golang.org/x/sync v0.20.0 // indirect
)
