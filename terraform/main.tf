module "key_rotator" {
  source = "./modules/key_rotator"

  name                = var.name
  lambda_zip          = var.lambda_zip
  handler             = var.handler
  runtime             = var.runtime
  rotation_days       = var.rotation_days
  secret_name         = var.secret_name
  tags                = var.tags
  key_group_name      = var.key_group_name
  lambda_memory_size  = var.lambda_memory_size
  lambda_timeout      = var.lambda_timeout
  key_retention_days       = var.key_retention_days
  min_public_keys_to_keep  = var.min_public_keys_to_keep
  only_delete_managed_keys = var.only_delete_managed_keys
}
