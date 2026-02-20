# External Integrations

**Analysis Date:** 2026-02-20

## APIs & External Services

**AWS DynamoDB:**
- Primary database — all entity storage
  - SDK: `github.com/aws/aws-sdk-go-v2/service/dynamodb` v1.55.0
  - Auth: `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` env vars
  - Local emulation: LocalStack at `AWS_ENDPOINT_URL=http://localhost:4566`
  - Client init: `internal/infrastructure/dynamo/client.go`

**AWS S3:**
- File/blob storage
  - SDK: `github.com/aws/aws-sdk-go-v2/service/s3` v1.53.0
  - Auth: same AWS credentials as DynamoDB
  - Bucket: `S3_BUCKET_NAME` env var (default: `go-api-files`)
  - Client init: `internal/infrastructure/s3/client.go`
  - Operations: Upload, Download, PresignedURL, Delete

**AWS SNS:**
- Outbound SMS delivery for phone verification OTPs
  - SDK: `github.com/aws/aws-sdk-go-v2/service/sns` v1.29.4
  - Auth: default AWS credential chain (does NOT use static keys from config, unlike S3/DynamoDB)
  - Region: `SNS_REGION` env var (default: `us-east-1`)
  - Client init: `internal/infrastructure/sns/sender.go`
  - Graceful fallback: `smsSender` is `nil` if init fails; phone confirm routes will error at call time

**SMTP (email):**
- Email delivery for password recovery OTPs and email confirmation tokens
  - Implementation: standard library `net/smtp`
  - Config: `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, `SMTP_USERNAME`, `SMTP_PASSWORD`
  - Client: `internal/infrastructure/smtp/mailer.go`
  - Auth: optional `smtp.PlainAuth` (skipped if `SMTP_USERNAME` is empty)
  - Local dev default: `localhost:1025` (mailhog-compatible)

## Data Storage

**Databases:**
- AWS DynamoDB (NoSQL)
  - Connection: `AWS_ENDPOINT_URL` (empty = real AWS; set for LocalStack)
  - Client: `aws-sdk-go-v2` with `attributevalue` marshalling
  - Tables: users, sessions, statuses, devices, notifications, files, user_verifications, app_versions
  - Bootstrap: `internal/infrastructure/dynamo/bootstrap.go` — creates tables + GSIs on startup

**File Storage:**
- AWS S3
  - Bucket configured via `S3_BUCKET_NAME`
  - Keys follow pattern: `{generated-path}/{filename}`

**Caching:**
- None

## Authentication & Identity

**Auth Provider:**
- Custom RS256 JWT (no third-party auth provider)
  - Implementation: `internal/infrastructure/jwt/provider.go`
  - Keys: RSA PEM files at `JWT_PRIVATE_KEY_PATH` / `JWT_PUBLIC_KEY_PATH`
  - Claims: `user_id`, `device_id`, `role`, `session_id`
  - Access token expiry: `JWT_EXPIRY_DAYS` (default 7)
  - Refresh token: random 32-byte hex stored in DynamoDB sessions table; expiry `REFRESH_TOKEN_EXPIRY_DAYS` (default 30)

## Monitoring & Observability

**Error Tracking:**
- None (no Sentry, Datadog, etc.)

**Logs:**
- `log/slog` (structured, JSON-friendly) for request logging and bootstrap events
- `log` (standard library) for startup/shutdown messages in `cmd/api/main.go`

## CI/CD & Deployment

**Hosting:**
- Not configured (no Dockerfile, no cloud deployment config detected)

**CI Pipeline:**
- None detected

## Environment Configuration

**Required env vars (non-defaulted):**
- `AWS_ACCESS_KEY_ID` — empty default; required for real AWS
- `AWS_SECRET_ACCESS_KEY` — empty default; required for real AWS
- `SMTP_USERNAME` / `SMTP_PASSWORD` — optional (plain auth skipped if empty)
- RSA key files at `JWT_PRIVATE_KEY_PATH` / `JWT_PUBLIC_KEY_PATH` — required; server exits if missing

**Secrets location:**
- `.env` file (gitignored)
- RSA PEM files at project root (gitignored via `*.pem` in `.gitignore`)

## Webhooks & Callbacks

**Incoming:**
- None

**Outgoing:**
- Email via SMTP (password recovery, email confirmation)
- SMS via AWS SNS (phone OTP verification)

---

*Integration audit: 2026-02-20*
