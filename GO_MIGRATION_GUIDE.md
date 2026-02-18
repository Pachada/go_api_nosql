# WSGI-Falcon API -> Go migration guide

This document summarizes how the current Python/Falcon API works so it can be recreated in Go from `openapi.yaml`.

## 1) Runtime architecture

- **Framework:** Falcon (`falcon.App`)
- **Server:** gevent WSGI on `0.0.0.0:3000` (`run.py`)
- **ORM:** SQLAlchemy with an active session middleware (`SQLAlchemySessionManager`)
- **Controller base:** `core\Controller.py` provides shared CRUD helpers and JSON envelope responses.
- **Route registration:** `@ROUTE_LOADER(...)` decorators in controller files, loaded at startup in `engine\Server.py`.
- **Config:** `config.ini` (routes, db, smtp, expiration, files, s3/sns).

## 2) Authentication and authorization

### JWT auth

- Middleware: `core\classes\middleware\Authenticator.py`.
- Header expected in Falcon `req.auth` (typically `Authorization: Bearer <token>`).
- JWT algorithm: **RS256** (`JWTUtils`), using local `private_key.pem` / `public_key.pem`.
- Token payload: `user_id`, `device_id`, `role_id`, `session_id`, plus `exp` (7 days).

### Auth skip rules

Requests bypass auth when:

1. Controller class has `skip_auth = True`, or
2. Method has `@Decorators.no_authorization_needed`.

### Role access rules

- Middleware also checks `models\Role.py::role_access`.
- If role has an allowlist and current controller name is not listed, request is forbidden.
- Additional action-level checks use Falcon hooks (`Hooks.check_privileges`) for admin-only operations.

## 3) Validation and response conventions

- `Hooks.post_validations` validates JSON body and required attributes.
- `Hooks.put_validations` requires `{id}` path param and valid JSON body.
- Generic list endpoints support:
  - `page` (default `1`)
  - `per_page` (default `50`)
  - dynamic query filters by model column name.
- Base response helper `Controller.response(...)` writes JSON with optional:
  - `message`
  - `error`
  - `error_code`

## 4) Controllers and endpoint behavior

## Public / auth-exempt endpoints

- `GET|POST /v1/health-check/{action}` (`action=ping`)
- `POST /v1/sessions/login`
- `POST /v1/users` (register + autologin)
- `POST /v1/password-recovery/{action}` (`request`, `validate-code`)
- `GET|POST /v1/test` (placeholder)

## Authenticated session endpoints

- `GET /v1/sessions` (current session with recursive user/device serialization)
- `POST /v1/sessions/logout`

## User endpoints

- `GET /v1/users`, `GET /v1/users/{id}`: admin-only.
- `PUT /v1/users/{id}`: user can modify own record; admin can modify any user.
- `DELETE /v1/users/{id}`: self or admin; user sessions are soft-deleted after user deletion.

## Password recovery

- `POST /v1/password-recovery/request`: send OTP by email or SMS.
- `POST /v1/password-recovery/validate-code`: validate OTP and create session/token.
- `POST /v1/password-recovery/change-password`: authenticated password change.

## Email confirmation

- `POST /v1/confirm-email/request`: send email confirmation token to current user.
- `POST /v1/confirm-email/validate-code`: body `{ "token": "..." }`.

## Roles and statuses

- Roles (`/v1/roles*`): fully admin-protected CRUD.
- Statuses (`/v1/statuses*`):
  - `GET` open to authenticated users
  - `POST|PUT|DELETE` admin-only
  - delete is hard delete (`delete()`), not soft delete.

## Devices and notifications

- Devices:
  - `GET /v1/devices`, `GET /v1/devices/{id}`, `PUT /v1/devices/{id}`
  - `DELETE /v1/devices/{id}` admin-only
  - `PUT /v1/devices/version` checks if device app version is outdated.
- Notifications:
  - `GET /v1/notifications*` returns unread records for current user.
  - `PUT /v1/notifications/{id}` marks as read.

## File endpoints

- Local storage: `/v1/files/local*`
- S3 storage: `/v1/files/s3*`
- Upload styles:
  1. multipart upload (`POST /.../local` or `/.../s3`)
  2. base64 JSON upload (`POST /.../base64`)
- Access rules:
  - private files can be accessed by uploader or admin.
  - delete allowed to uploader or admin.
- Query flags interpreted as string `"True"`:
  - `thumbnail`, `public`, `private`.

## 5) Core models to recreate in Go

These are the main domain entities used directly by controllers:

- `User`: identity, role, profile, verification flags.
- `Session`: links user and device; used as auth context target.
- `Role`: role constants and role access mapping.
- `Device`: user device UUID + push token + app version.
- `Status`: shared status catalog.
- `UserVerification`: OTP/email verification and KYC-like fields.
- `File`: metadata for local/S3 file objects.
- `PushNotificationSent`: user notification history (`readed` field name is intentional in current DB).
- `AppVersion`: current app version used by `/v1/devices/version`.

## 6) Go implementation blueprint

Recommended Go stack:

- Router: `chi` or `gin`
- DB: `gorm` or `sqlc` + migrations
- JWT: `golang-jwt/jwt/v5` with RS256
- Validation: `go-playground/validator`
- Config: `viper` or env-based config
- Storage abstraction: `FileStore` interface (`LocalStore`, `S3Store`)

Suggested layering:

1. `transport/http` (handlers, middleware)
2. `application` (use cases/services)
3. `domain` (entities)
4. `infrastructure` (db, smtp/sms, s3, jwt, queue)

Use middleware to replicate current behavior:

- `AuthMiddleware` for JWT verification and context injection.
- `RoleMiddleware` for role gating.
- Request validation middleware for required fields.

## 7) OpenAPI-to-Go plugin usage (recommended workflow)

1. Use the generated `openapi.yaml` from this repo as the source contract.
2. Generate Go server stubs with your plugin/tooling.
3. Keep generated handlers thin; call service-layer functions that implement the behavior documented above.
4. Add contract tests against `openapi.yaml` and regression tests for:
   - auth skip behavior
   - role restrictions
   - soft delete vs hard delete differences
   - password recovery + confirm-email flows

## 8) Known quirks to preserve during migration

- Some fields use legacy names (for example `readed`).
- Generic list filtering accepts any known model column as query param.
- Boolean query flags for file endpoints are parsed as exact string `"True"`.
- Sessions and many models use soft delete (`enable=0`) rather than physical delete.
