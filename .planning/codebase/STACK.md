# Technology Stack

**Analysis Date:** 2026-02-20

## Languages

**Primary:**
- Go 1.24.0 — all application code (`cmd/`, `internal/`)

## Runtime

**Environment:**
- Go runtime 1.24.0

**Package Manager:**
- Go modules (`go mod`)
- Lockfile: `go.sum` (present and committed)

## Frameworks

**Core:**
- `github.com/go-chi/chi/v5` v5.0.12 — HTTP router
- `github.com/go-chi/cors` v1.2.2 — CORS middleware

**Auth:**
- `github.com/golang-jwt/jwt/v5` v5.2.1 — RS256 JWT signing/verification

**Validation:**
- `github.com/go-playground/validator/v10` v10.30.1 — struct field validation via tags

**Testing:**
- None detected

**Build/Dev:**
- `air` (`.air.toml`) — hot-reload; builds to `./tmp/main.exe`

## Key Dependencies

**Critical:**
- `github.com/aws/aws-sdk-go-v2` v1.41.1 — AWS SDK core
- `github.com/aws/aws-sdk-go-v2/service/dynamodb` v1.55.0 — primary database
- `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue` v1.20.32 — DynamoDB ↔ struct marshalling
- `github.com/aws/aws-sdk-go-v2/service/s3` v1.53.0 — file storage
- `github.com/aws/aws-sdk-go-v2/service/sns` v1.29.4 — SMS delivery
- `golang.org/x/crypto` v0.48.0 — bcrypt password hashing
- `golang.org/x/time` v0.14.0 — token-bucket rate limiting

**Infrastructure:**
- `github.com/oklog/ulid/v2` v2.1.1 — ULID-based IDs (sortable, collision-safe)
- `github.com/joho/godotenv` v1.5.1 — `.env` file loading at startup
- `github.com/gabriel-vasile/mimetype` v1.4.12 — MIME type detection for file uploads

## Configuration

**Environment:**
- All config read from environment variables via `internal/config/config.go`
- `.env` loaded at startup via `godotenv` (falls back to OS env if missing)
- `.env.example` documents all variables with LocalStack defaults

**Key configs required:**
- `APP_PORT`, `APP_ENV`, `ALLOWED_ORIGINS`
- `AWS_REGION`, `AWS_ENDPOINT_URL`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`
- `DYNAMO_TABLE_*` (8 table names, see `.env.example`)
- `S3_BUCKET_NAME`
- `JWT_PRIVATE_KEY_PATH`, `JWT_PUBLIC_KEY_PATH`, `JWT_EXPIRY_DAYS`, `REFRESH_TOKEN_EXPIRY_DAYS`
- `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, `SMTP_USERNAME`, `SMTP_PASSWORD`
- `SNS_REGION`

**Build:**
- `.air.toml` — dev hot-reload config
- No Makefile or CI pipeline detected

## Platform Requirements

**Development:**
- Go 1.24+
- Docker (LocalStack: `infra/localstack/docker-compose.yml`)
- RSA key pair at `./private_key.pem` + `./public_key.pem` (gitignored, generated with `openssl`)

**Production:**
- Any platform running Go binaries
- AWS account with DynamoDB, S3, SNS enabled
- RSA key files mounted at configured paths

---

*Stack analysis: 2026-02-20*
