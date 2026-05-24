# Key Rotator Terraform module (skeleton)

locals {
  secret_name_effective = var.secret_name != "" ? var.secret_name : "/${var.name}/rotator"
}

resource "aws_iam_role" "lambda_role" {
  name = "${var.name}-lambda-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_basic" {
  role       = aws_iam_role.lambda_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_lambda_function" "rotator" {
  filename         = "${path.root}/${var.lambda_zip}"
  function_name    = "${var.name}-rotator"
  handler          = var.handler
  runtime          = var.runtime
  role             = aws_iam_role.lambda_role.arn
  source_code_hash = filebase64sha256("${path.root}/${var.lambda_zip}")
  memory_size      = var.lambda_memory_size
  timeout          = var.lambda_timeout
  environment {
    variables = {
      SECRET_NAME  = aws_secretsmanager_secret.rotator_secret.name
      NAME_PREFIX  = var.name
      KEY_GROUP_NAME = local.key_group_name_effective
      KEY_RETENTION_DAYS       = tostring(var.key_retention_days)
      MIN_PUBLIC_KEYS_TO_KEEP  = tostring(var.min_public_keys_to_keep)
      ONLY_DELETE_MANAGED_KEYS = tostring(var.only_delete_managed_keys)
    }
  }
}

resource "aws_lambda_permission" "allow_secretsmanager" {
  statement_id  = "AllowExecutionFromSecretsManager"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.rotator.function_name
  principal     = "secretsmanager.amazonaws.com"
  source_arn    = aws_secretsmanager_secret.rotator_secret.arn
}

resource "aws_secretsmanager_secret_rotation" "rotator_rotation" {
  secret_id            = aws_secretsmanager_secret.rotator_secret.id
  rotation_lambda_arn  = aws_lambda_function.rotator.arn
  rotation_rules {
    automatically_after_days = var.rotation_days
  }

  depends_on = [aws_lambda_permission.allow_secretsmanager]
}

# Secrets Manager secret created by infra; Lambda will populate secret values (current/previous) at runtime.
resource "aws_secretsmanager_secret" "rotator_secret" {
  name        = local.secret_name_effective
  description = "Secret for key rotator: stores current and previous private keys and metadata"
  tags        = var.tags
}

locals {
  # Effective KeyGroup name: prefer caller-supplied var, otherwise default derived from module name
  key_group_name_effective = var.key_group_name != "" ? var.key_group_name : "${var.name}-key-group"

  secrets_stmt = {
    Effect = "Allow"
    Action = [
      "secretsmanager:GetSecretValue",
      "secretsmanager:PutSecretValue",
      "secretsmanager:DescribeSecret",
      "secretsmanager:UpdateSecretVersionStage"
    ]
    Resource = [aws_secretsmanager_secret.rotator_secret.arn]
  }

  cloudfront_stmt = {
    Effect = "Allow"
    Action = [
      "cloudfront:ListPublicKeys",
      "cloudfront:GetPublicKey",
      "cloudfront:CreatePublicKey",
      "cloudfront:DeletePublicKey",
      "cloudfront:ListKeyGroups",
      "cloudfront:CreateKeyGroup",
      "cloudfront:GetKeyGroup",
      "cloudfront:UpdateKeyGroup"
    ]
    Resource = ["*"]
  }

  sts_stmt = {
    Effect   = "Allow"
    Action   = ["sts:GetCallerIdentity"]
    Resource = ["*"]
  }

  statements = concat(
    [local.secrets_stmt, local.cloudfront_stmt, local.sts_stmt]
  )

  lambda_policy = {
    Version   = "2012-10-17"
    Statement = local.statements
  }
}

resource "aws_iam_role_policy" "lambda_policy" {
  name = "${var.name}-lambda-policy"
  role = aws_iam_role.lambda_role.id

  policy = jsonencode(local.lambda_policy)
}
