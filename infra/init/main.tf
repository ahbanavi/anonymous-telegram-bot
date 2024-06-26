provider "aws" {
  region = var.aws_region
}

resource "aws_s3_bucket" "terraform_state" {
  bucket = var.terraform_state_bucket
}

resource "aws_dynamodb_table" "terraform_lock" {
  name         = var.terraform_state_lock_dynamodb_table
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "LockID"

  attribute {
    name = "LockID"
    type = "S"
  }

  tags = {
    Name = "TerraformStateLocking"
  }
}

# S3 bucket to store lambda function codes
resource "aws_s3_bucket" "lambda_bucket" {
  bucket = var.lambda_bucket
}