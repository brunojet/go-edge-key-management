variable "name" {
  type = string
}

variable "lambda_zip" {
  type = string
}

variable "handler" {
  type    = string
  default = "rotator"
}

variable "runtime" {
  type    = string
  default = "provided.al2023"
}

variable "schedule_expression" {
  type    = string
  default = "rate(7 days)"
}

variable "secret_name" {
  type = string
  default = ""
}

variable "kms_key_arn" {
  type    = string
  default = ""
}

variable "key_group_id" {
  type    = string
  default = ""
}

variable "key_group_name" {
  type    = string
  default = ""
}

variable "initial_public_key_ids" {
  type    = list(string)
  default = []
}

variable "lambda_memory_size" {
  type    = number
  default = 256
}

variable "lambda_timeout" {
  type    = number
  default = 60
}

variable "tags" {
  type    = map(string)
  default = {}
}
