# Codebase Concerns

**Analysis Date:** 2026-02-20 | **Last updated:** 2026-02-20

## Tech Debt

**`ScanPage` uses DynamoDB full-table Scan:**
- Issue: `internal/infrastructure/dynamo/users.go` `ScanPage` uses `Scan` with a `FilterExpression` on `enable`. DynamoDB applies the filter *after* reading pages, so it consumes read capacity for deleted items and may return fewer than `limit` items per page.
- Files: `internal/infrastructure/dynamo/users.go` (line 93)
- Impact: Inefficient at scale; admin list endpoint (`GET /v1/users`) degrades as table grows
- Fix approach: Add a GSI on `enable` (or use a sparse GSI pattern) and switch to `Query`

## Known Bugs

**Phone password recovery silently returns error:**
- Symptoms: `POST /v1/password-recovery/{action}` with `phone_number` (no email) returns 400 "phone recovery not supported; provide email"
- Files: `internal/application/auth/service.go`
- Trigger: Any client that sends `phone_number` instead of `email` in recovery request
- Workaround: Always use `email` field for password recovery

## Security Considerations

**Rate limiter state is in-process only:**
- Risk: Per-IP rate limits (`middleware/ratelimit.go`) are stored in a `sync.Map` in memory. State is lost on restart and not shared across multiple instances.
- Files: `internal/transport/http/middleware/ratelimit.go`
- Current mitigation: Comment in code notes this; suggests API Gateway / WAF as primary layer
- Recommendations: Configure API Gateway request throttling or AWS WAF rate-based rules as the authoritative rate limit. The in-process limiter is a secondary defence only.

**X-Forwarded-For can be spoofed:**
- Risk: `realIP()` reads `X-Forwarded-For` to identify client IP for rate limiting. If the API is reachable directly (not behind a trusted proxy), clients can spoof this header to bypass per-IP limits.
- Files: `internal/transport/http/middleware/ratelimit.go` (line 97)
- Current mitigation: Security note in comment; API Gateway is recommended as the proxy
- Recommendations: Never expose the Go server directly to the internet; always front with API Gateway or a trusted reverse proxy

**OTP/token values sent in plain-text email/SMS body:**
- Risk: OTP codes are included directly in the email/SMS body string without any formatting or expiry reminder.
- Files: `internal/application/auth/service.go` (lines 161, 242, 283)
- Current mitigation: `subtle.ConstantTimeCompare` used for OTP validation (prevents timing attacks)
- Recommendations: Consider adding expiry time to the message and rate-limiting OTP request frequency

## Performance Bottlenecks

**DynamoDB Scan for user list:**
- Problem: `GET /v1/users` (admin only) scans the entire users table to filter enabled users
- Files: `internal/infrastructure/dynamo/users.go` (`ScanPage`)
- Cause: `Scan` with `FilterExpression` reads all items then filters; consumes full RCU regardless of filter matches
- Improvement path: Add sparse GSI on `enable` attribute, or store active/deleted in separate tables

## Fragile Areas

**`main.go` infrastructure wiring:**
- Files: `cmd/api/main.go`, `internal/transport/http/router.go`
- Why fragile: All dependencies are manually wired in two places (main.go for infra init, router.go for service construction). Adding a new entity requires touching both files.
- Safe modification: Follow the existing pattern — init client in `main.go`, add to `Deps` struct, instantiate service and handler in `router.go`
- Test coverage: None

**DynamoDB bootstrap on startup:**
- Files: `internal/infrastructure/dynamo/bootstrap.go`
- Why fragile: `Bootstrap()` calls `CreateTable` for all 8 tables every startup. On real AWS, table creation is eventually consistent and `Bootstrap` doesn't wait for `ACTIVE` status before the server starts accepting requests.
- Safe modification: Only add new tables/GSIs; never remove (existing data would orphan). For GSI additions on existing tables, run `UpdateTable` manually first.
- Test coverage: None

## Scaling Limits

**In-process rate limiter:**
- Current capacity: Single Go instance; state in-memory
- Limit: No cross-instance rate limiting; cold starts reset all limits
- Scaling path: Replace with Redis-backed rate limiter or rely solely on API Gateway throttling

## Dependencies at Risk

**`net/smtp` (standard library):**
- Risk: No HTML email support, no retry logic
- Impact: Email delivery reliability in production (TLS now enforced via `SMTP_TLS=true`)
- Migration plan: Switch to AWS SES SDK or a transactional email provider (SendGrid, Postmark)

## Missing Critical Features

**Test coverage gaps:**
- **Status: Partially resolved** — 27 unit tests across 6 files (see TESTING.md). Middleware fully covered.
- Remaining gaps: HTTP handlers (`internal/transport/http/handler/`), application services (device, file, notification, session, status), DynamoDB repo integration tests.
- Recommended next steps:
  1. Handler tests for `internal/transport/http/handler/` (authorization logic, response shapes)
  2. Integration tests for DynamoDB repos against LocalStack

**Phone-based password recovery not implemented:**
- Problem: `RequestPasswordRecovery` returns `ErrBadRequest` when `phone_number` is provided
- Files: `internal/application/auth/service.go`
- Blocks: SMS-only users cannot recover their accounts

## Test Coverage Gaps

**Application services:**
- What's not tested: `internal/application/device`, `file`, `notification`, `session`, `status` service files
- Files: `internal/application/*/service.go` (device, file, notification, session, status)
- Risk: Regressions in these service flows go undetected
- Priority: Medium

**DynamoDB helper functions:**
- Remaining: cursor encode/decode in `internal/infrastructure/dynamo/users.go`
- Priority: Medium

**HTTP handlers:**
- What's not tested: All of `internal/transport/http/handler/`
- Files: `internal/transport/http/handler/*.go`
- Risk: Authorization logic bugs (e.g., self-update vs admin-update checks in `users.go`)
- Priority: Medium

---

*Concerns audit: 2026-02-20 | Updated: 2026-02-20*
