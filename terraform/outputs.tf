output "lambda_arn" {
  value = module.key_rotator.lambda_arn
}

output "lambda_function_name" {
  value = module.key_rotator.lambda_function_name
}

output "secret_arn" {
  value = module.key_rotator.secret_arn
}

output "secret_name" {
  value = module.key_rotator.secret_name
}

output "lambda_role_arn" {
  value = module.key_rotator.lambda_role_arn
}
