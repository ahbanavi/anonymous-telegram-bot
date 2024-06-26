variable "aws_region" {
  description = "The AWS region to create resources in"
  type        = string
}

variable "terraform_state_bucket" {
  description = "The AWS S3 bucket used to store terraform state"
  type        = string
}

variable "terraform_state_lock_dynamodb_table" {
  description = "The AWS DynamoDB table name to be used for terraform state lock"
  type        = string
  default     = "anonymous-chat-terraform-state-lock"
}

variable "lambda_bucket" {
  description = "The AWS S3 bucket for storing lambda function codes"
  type        = string
}