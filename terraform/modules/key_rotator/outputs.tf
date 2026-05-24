output "lambda_arn" {
  value = aws_lambda_function.rotator.arn
}

output "lambda_function_name" {
  value = aws_lambda_function.rotator.function_name
}

output "secret_arn" {
  value = aws_secretsmanager_secret.rotator_secret.arn
}

output "secret_name" {
  value = aws_secretsmanager_secret.rotator_secret.name
}

output "lambda_role_arn" {
  value = aws_iam_role.lambda_role.arn
}

output "key_group_id" {
  value       = local.key_group_id_effective
  description = "CloudFront KeyGroup ID used by the rotator (module creates one if var.key_group_id is empty)"
}

output "key_group_name" {
  value       = local.key_group_name_effective
  description = "CloudFront KeyGroup name used by the rotator (module creates one if var.key_group_name is empty)"
}
