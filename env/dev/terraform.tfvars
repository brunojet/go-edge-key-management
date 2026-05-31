name = "go-edge"
lambda_zip = "../build/rotator.zip"
secret_name = "/go-edge-key-management/rotator"
rotation_days = 7
tags = {
  environment = "dev"
  project     = "go-edge-key-management"
}

# Optionally specify a name for the KeyGroup created by Terraform (if left empty,
# the module defaults to "${var.name}-key-group").
key_group_name = ""

# Optional: initial public key IDs to include when Terraform should create the KeyGroup.
# If you want Terraform to create the KeyGroup, supply at least one public key ID here.
initial_public_key_ids = []

# Lambda runtime resource sizing
lambda_memory_size = 128
lambda_timeout = 15

# Key retention settings for the rotator
key_retention_days = 30
min_public_keys_to_keep = 2
only_delete_managed_keys = true

