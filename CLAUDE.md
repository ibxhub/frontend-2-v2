# CLAUDE.md

Guidance for Claude Code (claude.ai/code) when working with this repository.

## What this app does

File transfer tool for Infoblox TME: upload a file via a multipart form, list files, download any of them back. Authenticated with Okta OIDC. Files are stored in S3 in production and on local disk for development.

## Commands

```bash
# Run locally (requires JWT_SIGNING_KEY; DEV_AUTH_BYPASS=true skips Okta and auto-logs you in)
JWT_SIGNING_KEY=$(openssl rand -base64 32) DEV_AUTH_BYPASS=true go run .

# Run tests with coverage
go test ./... -cover

# Build
go build -o frontend-2-v2 .

# Tidy dependencies
go mod tidy
```

## Architecture

Server-side rendered Go web app using `net/http` and `html/template` (templates embedded via `embed.FS`).

```
main.go                            wiring: config → store → handlers → mux
handlers/files.go                  Home, Upload, Download
internal/auth/                     JWT sign/verify, Bearer/cookie middleware, Okta OIDC
internal/config/                   env-driven config with secrets.json fallback
internal/storage/                  Store interface, Disk impl, S3 impl
templates/                         embedded HTML
static/                            CSS + JS (auth-aware fetch wrappers)
infra/                             Terraform: S3, KMS, IAM, Secrets Manager
```

### Routes

| Method | Path             | Auth | Purpose                                                |
| ------ | ---------------- | ---- | ------------------------------------------------------ |
| GET    | `/healthz`       | no   | Liveness probe                                         |
| GET    | `/auth/login`    | no   | Start Okta OIDC (or mint a dev JWT if `DEV_AUTH_BYPASS`) |
| GET    | `/auth/callback` | no   | Okta callback → mint app JWT → redirect to `/?jwt=…`   |
| GET    | `/auth/logout`   | no   | Clear JWT cookie, redirect to login                    |
| GET    | `/`              | yes  | Page with upload form and file list                    |
| POST   | `/upload`        | yes  | Multipart upload (≤25 MB)                              |
| GET    | `/download/{name}` | yes | Stream a file as attachment                            |

### Auth flow

1. Frontend page loads. JS captures `?jwt=` from the URL, stores it in `localStorage`, and strips it from the URL.
2. Every API call (upload, download) goes through `fetch` with `Authorization: Bearer <jwt>`.
3. Middleware accepts the token from `Authorization`, the `?jwt=` query param, or a `jwt` cookie (set on login).
4. 401 clears `localStorage` and redirects to `/auth/login`.

### Storage

`internal/storage.Store` interface with two implementations:

- `storage.Disk` — `uploads/` on local filesystem. Used when `UPLOADS_BUCKET` is empty.
- `storage.S3` — KMS-encrypted S3. Used when `UPLOADS_BUCKET` is set. `NewS3WithEndpoint` is provided for tests pointing at a fake S3.

### Configuration

`internal/config.Load` resolves (highest to lowest priority):

1. Environment variables (`JWT_SIGNING_KEY`, `UPLOADS_BUCKET`, `OKTA_*`, etc.)
2. `secrets.json` at the working directory (gitignored; see `secrets.json.example`)
3. Built-in defaults

`JWT_SIGNING_KEY` is required. The pipeline injects all other values from AWS Secrets Manager (`frontend-2-v2/<env>/config`).

### Infrastructure

`infra/` contains Terraform for the AWS resources this app needs: S3 bucket (KMS-encrypted, versioned, private), KMS key, IAM role (least privilege), Secrets Manager entry. See `infra/README.md`.

## Pre-ship checklist

Per the ABCD playbook, before submitting to CI/CD:

- Branch is `feature/*` (Dev) or `main` (Prod release).
- `go build ./...` clean.
- `go vet ./...` clean.
- `go test ./... -cover` — all packages with logic ≥70%.
- No secrets in code (`gitleaks` or equivalent).
- No critical CVEs (`govulncheck ./...`).
- Server boots and `GET /healthz` returns 200.
