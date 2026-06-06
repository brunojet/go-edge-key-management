variable "name" {
  type = string
}

variable "lambda_zip" {
  type = string
}

variable "handler" {
  type    = string
  default = "bootstrap"
}

variable "runtime" {
  type    = string
  default = "provided.al2"
}

variable "rotation_days" {
  type    = number
  default = 7
}

variable "secret_name" {
  type    = string
  default = ""
}


variable "key_group_name" {
  type    = string
  default = ""
}

variable "lambda_memory_size" {
  type    = number
  default = 128
}

variable "lambda_timeout" {
  type    = number
  default = 30
}

variable "tags" {
  type    = map(string)
  default = {}
}

variable "aws_region" {
  type    = string
  default = "us-east-1"
}

variable "key_retention_days" {
  type    = number
  default = 30
}

variable "min_public_keys_to_keep" {
  type    = number
  default = 2
}

variable "only_delete_managed_keys" {
  type    = bool
  default = true
}
