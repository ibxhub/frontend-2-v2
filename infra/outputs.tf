output "uploads_bucket_name" {
  description = "S3 bucket the app reads and writes to."
  value       = aws_s3_bucket.uploads.bucket
}

output "uploads_bucket_arn" {
  description = "ARN of the uploads bucket."
  value       = aws_s3_bucket.uploads.arn
}

output "kms_key_arn" {
  description = "KMS key used to encrypt bucket objects and the runtime secret."
  value       = aws_kms_key.uploads.arn
}

output "app_role_arn" {
  description = "IAM role the runtime assumes (Lambda execution role)."
  value       = aws_iam_role.app.arn
}

output "config_secret_arn" {
  description = "Secrets Manager ARN for runtime config (Okta + JWT signing key)."
  value       = aws_secretsmanager_secret.app.arn
}
