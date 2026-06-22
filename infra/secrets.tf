resource "aws_secretsmanager_secret" "app" {
  name        = "${var.app_name}/${var.environment}/config"
  description = "Runtime configuration for ${var.app_name} (${var.environment}): Okta + JWT signing key."
  kms_key_id  = aws_kms_key.uploads.arn

  recovery_window_in_days = 30
}

resource "aws_secretsmanager_secret_version" "app_placeholder" {
  secret_id = aws_secretsmanager_secret.app.id
  secret_string = jsonencode({
    jwt_signing_key    = "REPLACE_ME"
    okta_issuer        = "REPLACE_ME"
    okta_client_id     = "REPLACE_ME"
    okta_client_secret = "REPLACE_ME"
    okta_redirect_url  = "REPLACE_ME"
  })

  lifecycle {
    ignore_changes = [secret_string]
  }
}
