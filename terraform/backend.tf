// Terraform backend configuration for S3 remote state.
// Uses existing S3 bucket 'brunojet-tfstate'. Adjust `region` if necessary.

terraform {
  backend "s3" {
    bucket       = "brunojet-tfstate"
    key          = "go-edge-key-management/terraform.tfstate"
    region       = "us-east-1"
    encrypt      = true
    use_lockfile = true
  }
}
