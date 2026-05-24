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
      KEY_GROUP_ID   = local.key_group_id_effective
      KEY_GROUP_NAME = local.key_group_name_effective
    }
  }
}

resource "aws_cloudwatch_event_rule" "schedule" {
  name                = "${var.name}-rotator-schedule"
  schedule_expression = var.schedule_expression
}

resource "aws_cloudwatch_event_target" "lambda_target" {
  rule      = aws_cloudwatch_event_rule.schedule.name
  target_id = "Lambda"
  arn       = aws_lambda_function.rotator.arn
}

resource "aws_lambda_permission" "allow_event" {
  statement_id  = "AllowExecutionFromCloudWatch"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.rotator.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.schedule.arn
}

# Secrets Manager secret created by infra; Lambda will populate secret values (current/previous) at runtime.
resource "aws_secretsmanager_secret" "rotator_secret" {
  name        = local.secret_name_effective
  description = "Secret for key rotator: stores current and previous private keys and metadata"
  kms_key_id  = var.kms_key_arn != "" ? var.kms_key_arn : null
  tags        = var.tags
}

# Optionally create a CloudFront KeyGroup if the caller did not supply one.
resource "aws_cloudfront_key_group" "rotator_key_group" {
  count = var.key_group_id == "" && length(var.initial_public_key_ids) > 0 ? 1 : 0
  name  = var.key_group_name != "" ? var.key_group_name : "${var.name}-key-group"
  items = var.initial_public_key_ids

  lifecycle {
    # Allow the Lambda to add/remove public keys at runtime without Terraform attempting to revert
    ignore_changes = [items]
  }
}

locals {
  # Effective KeyGroup ID: prefer caller-supplied var, otherwise the optionally-created resource
  key_group_id_effective = var.key_group_id != "" ? var.key_group_id : (length(aws_cloudfront_key_group.rotator_key_group) > 0 ? aws_cloudfront_key_group.rotator_key_group[0].id : "")
  # Effective KeyGroup name: prefer caller-supplied var, otherwise derive from created resource or default
  key_group_name_effective = var.key_group_name != "" ? var.key_group_name : (length(aws_cloudfront_key_group.rotator_key_group) > 0 ? aws_cloudfront_key_group.rotator_key_group[0].name : "${var.name}-key-group")

  secrets_stmt = {
    Effect = "Allow"
    Action = [
      "secretsmanager:GetSecretValue",
      "secretsmanager:PutSecretValue",
      "secretsmanager:DescribeSecret"
    ]
    Resource = [aws_secretsmanager_secret.rotator_secret.arn]
  }

  cloudfront_stmt = {
    Effect = "Allow"
    Action = [
      "cloudfront:CreatePublicKey",
      "cloudfront:ListPublicKeys",
      "cloudfront:GetPublicKey",
      "cloudfront:CreateKeyGroup",
      "cloudfront:GetKeyGroup",
      "cloudfront:UpdateKeyGroup"
    ]
    Resource = ["*"]
  }

  kms_stmt = {
    Effect = "Allow"
    Action = [
      "kms:Decrypt",
      "kms:Encrypt",
      "kms:GenerateDataKey",
      "kms:DescribeKey"
    ]
    Resource = [var.kms_key_arn]
  }

  sts_stmt = {
    Effect   = "Allow"
    Action   = ["sts:GetCallerIdentity"]
    Resource = ["*"]
  }

  statements = concat(
    [local.secrets_stmt, local.cloudfront_stmt, local.sts_stmt],
    var.kms_key_arn != "" ? [local.kms_stmt] : []
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
