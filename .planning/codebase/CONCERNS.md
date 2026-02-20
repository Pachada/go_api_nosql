# Codebase Concerns

**Analysis Date:** 2026-02-20

## Tech Debt

**Concrete infrastructure types in `Deps` struct:**
- Issue: `internal/transport/http/router.go` `Deps` struct uses concrete `*dynamo.UserRepo`, `*dynamo.SessionRepo`, etc. instead of the same small interfaces the services define. This binds the transport wiring to DynamoDB specifically.
- Files: `internal/transport/http/router.go` (lines 32–44)
- Impact: Cannot swap DynamoDB for another store without editing `router.go`; harder to test router wiring
- Fix approach: Change `Deps` fields to the same interface types already defined in each `application/` package

**`ScanPage` uses DynamoDB full-table Scan:**
- Issue: `internal/infrastructure/dynamo/users.go` `ScanPage` uses `Scan` with a `FilterExpression` on `enable`. DynamoDB applies the filter *after* reading pages, so it consumes read capacity for deleted items and may return fewer than `limit` items per page.
- Files: `internal/infrastructure/dynamo/users.go` (line 93)
- Impact: Inefficient at scale; admin list endpoint (`GET /v1/users`) degrades as table grows
- Fix approach: Add a GSI on `enable` (or use a sparse GSI pattern) and switch to `Query`

**`buildUpdateExpr` iterates a `map[string]interface{}` with non-deterministic order:**
- Issue: ~~`internal/infrastructure/dynamo/helpers.go` builds the `SET` expression from a Go map. Map iteration order is random, so the generated expression string varies across calls (though DynamoDB semantics are unaffected).~~
- **Status: Fixed** — keys are now sorted with `sort.Strings` before building the expression.
- Files: `internal/infrastructure/dynamo/helpers.go`

**Typo: `fieldReaded` constant:**
- Issue: `internal/infrastructure/dynamo/fields.go` defines `fieldReaded = "readed"`. Correct English is "read" (past tense is same as present).
- Files: `internal/infrastructure/dynamo/fields.go` (line 8)
- Impact: The DynamoDB attribute is named `readed` in storage; renaming requires a data migration
- Fix approach: Leave the DynamoDB attribute name as-is for now; rename the Go constant to `fieldRead` and update callers

## Known Bugs

**Phone password recovery silently returns error:**
- Symptoms: `POST /v1/password-recovery/{action}` with `phone_number` (no email) returns 400 "phone recovery not supported; provide email"
- Files: `internal/application/auth/service.go` (line 141)
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

**SMTP sends plain text emails without TLS enforcement:**
- Risk: `net/smtp` is used with `smtp.PlainAuth`. If `SMTP_USERNAME` is set and the connection is not TLS-wrapped, credentials may be sent in plaintext.
- Files: `internal/infrastructure/smtp/mailer.go`
- Current mitigation: None
- Recommendations: Use `smtp.Dial` with `StartTLS` or use a TLS-wrapped connection. For production, consider a managed email service (SES, SendGrid) instead.

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
- Risk: No HTML email support, no retry logic, no TLS enforcement by default
- Impact: Email delivery reliability and security in production
- Migration plan: Switch to AWS SES SDK or a transactional email provider (SendGrid, Postmark)

## Missing Critical Features

**Zero test coverage:**
- ~~Problem: No `*_test.go` files exist anywhere in the codebase~~
- **Status: Partially resolved** — 27 unit tests added across 6 files (see TESTING.md).
- Remaining gaps: HTTP handlers (`internal/transport/http/handler/`), remaining application services (device, file, notification, session, status), DynamoDB repo integration tests.
- Recommended next steps:
  1. Handler tests for `internal/transport/http/handler/` (authorization logic, response shapes)
  2. Integration tests for DynamoDB repos against LocalStack

**No health check that tests dependencies:**
- Problem: `GET /v1/health-check/ping` returns static `"pong"` without checking DynamoDB or S3 connectivity
- Files: `internal/transport/http/handler/health.go`
- Blocks: Cannot use health check for load balancer / readiness probe accurately

**Phone-based password recovery not implemented:**
- Problem: `RequestPasswordRecovery` returns `ErrBadRequest` when `phone_number` is provided
- Files: `internal/application/auth/service.go` (line 141)
- Blocks: SMS-only users cannot recover their accounts

## Test Coverage Gaps

**Application services:**
- What's not tested: `internal/application/device`, `file`, `notification`, `session`, `status` service files
- Files: `internal/application/*/service.go` (device, file, notification, session, status)
- Risk: Regressions in these service flows go undetected
- Priority: Medium

**DynamoDB helper functions:**
- ~~What's not tested: `buildUpdateExpr`, cursor encode/decode in `users.go`~~
- **`buildUpdateExpr`: Covered** (`internal/infrastructure/dynamo/helpers_test.go`)
- Remaining: cursor encode/decode in `internal/infrastructure/dynamo/users.go`
- Priority: Medium

**HTTP handlers:**
- What's not tested: All of `internal/transport/http/handler/`
- Files: `internal/transport/http/handler/*.go`
- Risk: Authorization logic bugs (e.g., self-update vs admin-update checks in `users.go`)
- Priority: Medium

**Middleware:**
- ~~What's not tested: `auth.go`, `role.go`, `ratelimit.go`~~
- **Status: Covered** (`auth_test.go`, `role_test.go`, `ratelimit_test.go` — 12 tests)
- Priority: Done

---

*Concerns audit: 2026-02-20*
