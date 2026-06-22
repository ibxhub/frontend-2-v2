# Infrastructure (Terraform)

This module provisions the AWS resources required by `frontend-2-v2`.

## What it creates

- **S3 bucket** for uploaded files — private (public access blocked), versioned, KMS-encrypted (`aws:kms`), with a lifecycle rule that expires noncurrent versions after 90 days.
- **KMS key + alias** dedicated to this app's uploads bucket and runtime secret. Rotation enabled.
- **Secrets Manager entry** at `frontend-2-v2/<env>/config` holding `jwt_signing_key`, `okta_issuer`, `okta_client_id`, `okta_client_secret`, `okta_redirect_url`. Initial values are `REPLACE_ME` placeholders — the **Platform Admin** must fill them after the first apply.
- **IAM role** (Lambda assume) with a least-privilege inline policy: object RW + bucket list scoped to this bucket, KMS Encrypt/Decrypt scoped to this key, read on the runtime secret, and CloudWatch log writes.

## Variables

| Name          | Default                | Description                                |
| ------------- | ---------------------- | ------------------------------------------ |
| `app_name`    | `frontend-2-v2`           | Prefix for resource names                  |
| `environment` | `dev`                  | One of `dev`, `stage`, `prod`              |
| `aws_region`  | `us-east-1`            | AWS region                                 |
| `owner`       | `snayak@infoblox.com`  | Owner tag                                  |
| `bucket_name` | (derived)              | Override if a specific bucket name is required |

## Outputs

`uploads_bucket_name`, `uploads_bucket_arn`, `kms_key_arn`, `app_role_arn`, `config_secret_arn`.

## How the app reads config at runtime

The runtime resolves config in this order (highest priority first):

1. Environment variables (`UPLOADS_BUCKET`, `JWT_SIGNING_KEY`, `OKTA_*`, etc.) — set by the deployment.
2. `secrets.json` at the working directory — local dev only, gitignored.

In production the ABCD pipeline reads `config_secret_arn` from Secrets Manager and injects the values as env vars. Local dev uses `secrets.json` from the project root.

## Note for Platform Admin

After the first `terraform apply`, populate the runtime secret with real values:

```
aws secretsmanager put-secret-value \
  --secret-id frontend-2-v2/dev/config \
  --secret-string '{"jwt_signing_key":"<32+ random bytes base64>","okta_issuer":"https://<tenant>.okta.com/oauth2/default","okta_client_id":"...","okta_client_secret":"...","okta_redirect_url":"https://<host>/auth/callback"}'
```

The `aws_secretsmanager_secret_version.app_placeholder` resource has `lifecycle.ignore_changes = [secret_string]` so subsequent `terraform apply` runs do not overwrite the real values.
