name = "go-edge"
lambda_zip = "../build/rotator.zip"
secret_name = "/go-edge-key-management/rotator"
kms_key_arn = ""
schedule_expression = "rate(7 days)"
tags = {
  environment = "dev"
  project     = "go-edge-key-management"
}

# Optionally specify an existing KeyGroup ID to use (leave empty to let the module create one)
key_group_id = ""

# Optionally specify a name for the KeyGroup created by Terraform (if left empty,
# the module defaults to "${var.name}-key-group").
key_group_name = ""

# Optional: initial public key IDs to include when Terraform should create the KeyGroup.
# If you want Terraform to create the KeyGroup, supply at least one public key ID here.
initial_public_key_ids = []

# Lambda runtime resource sizing
lambda_memory_size = 256
lambda_timeout = 60
