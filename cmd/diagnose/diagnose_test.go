package main

import "testing"

func TestServiceHost(t *testing.T) {
	tests := []struct {
		service string
		region  string
		want    string
	}{
		{"cloudfront", "us-east-1", "cloudfront.amazonaws.com"},
		{"cloudfront", "eu-west-1", "cloudfront.amazonaws.com"},
		{"secretsmanager", "us-east-1", "secretsmanager.us-east-1.amazonaws.com"},
		{"secretsmanager", "eu-west-1", "secretsmanager.eu-west-1.amazonaws.com"},
		{"sts", "us-east-1", "sts.us-east-1.amazonaws.com"},
		{"sts", "ap-southeast-1", "sts.ap-southeast-1.amazonaws.com"},
	}

	for _, tt := range tests {
		t.Run(tt.service+"/"+tt.region, func(t *testing.T) {
			got := serviceHost(tt.service, tt.region)
			if got != tt.want {
				t.Errorf("serviceHost(%q, %q) = %q, want %q", tt.service, tt.region, got, tt.want)
			}
		})
	}
}
