module "key_rotator" {
  source = "./modules/key_rotator"

  name                = var.name
  lambda_zip          = var.lambda_zip
  handler             = var.handler
  runtime             = var.runtime
  schedule_expression = var.schedule_expression
  secret_name         = var.secret_name
  kms_key_arn         = var.kms_key_arn
  tags                = var.tags
  key_group_id        = var.key_group_id
  key_group_name      = var.key_group_name
  lambda_memory_size  = var.lambda_memory_size
  lambda_timeout      = var.lambda_timeout
}
