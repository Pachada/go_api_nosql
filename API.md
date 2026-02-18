# REST API Reference

Go REST API backed by **AWS DynamoDB** (multi-table) and **S3** for file storage.

Base URL: `http://localhost:3000`  
All resource paths are prefixed with `/v1`.

---

## Authentication

Protected endpoints require an `Authorization: Bearer <token>` header.  
Tokens are **RS256 JWTs** valid for 7 days (configurable via `JWT_EXPIRY_DAYS`).  
A token is returned by **login** (`POST /v1/sessions/login`) and **register** (`POST /v1/users`).

JWT payload fields:

| Field        | Description                       |
|-------------|-----------------------------------|
| `user_id`   | UUID of the authenticated user    |
| `device_id` | UUID of the device used to log in |
| `role_id`   | UUID of the user's role           |
| `session_id`| UUID of the active session        |

---

## Response envelopes

Every endpoint returns JSON. Common wrapper shapes:

### `MessageEnvelope`
```json
{ "message": "ok", "error": "", "error_code": 0 }
```

### `AuthEnvelope`
```json
{
  "Bearer": "<jwt>",
  "session": { ... },
  "message": ""
}
```

### `SessionEnvelope`
```json
{ "session": { ... } }
```

### `PaginatedUsersEnvelope`
```json
{
  "max_page": 5,
  "actual_page": 1,
  "per_page": 50,
  "data": [ { ... } ]
}
```

---

## Health

### `GET /v1/health-check/{action}` · `POST /v1/health-check/{action}`
No auth required. `action` must be `ping`.

**Response `200`**
```json
{ "message": "pong" }
```

### `GET /v1/test` · `POST /v1/test`
No auth required. Placeholder endpoint for connectivity checks.

---

## Sessions

### `POST /v1/sessions/login` — Login
No auth required.

**Request body**
```json
{
  "username": "johndoe",
  "password": "secret",
  "device_uuid": "optional-device-uuid"
}
```
`username` also accepts an email address. `device_uuid` is optional; if omitted a new device is created automatically.

**Response `200`** — `AuthEnvelope`
```json
{
  "Bearer": "<jwt>",
  "session": {
    "id": "uuid",
    "user_id": "uuid",
    "device_id": "uuid",
    "enable": true,
    "created": "2024-01-01T00:00:00Z",
    "updated": "2024-01-01T00:00:00Z",
    "user": { ... }
  }
}
```

**Errors:** `401 invalid credentials`, `401 account disabled`

---

### `GET /v1/sessions` — Get current session
🔒 Requires auth.

**Response `200`** — `SessionEnvelope` with embedded `user` object.

---

### `POST /v1/sessions/logout` — Logout
🔒 Requires auth. Soft-disables the current session.

**Response `200`** — `MessageEnvelope`

---

## Users

### `POST /v1/users` — Register
No auth required. Creates the user **and** auto-starts a session.

**Request body**
```json
{
  "username": "johndoe",
  "password": "secret",
  "email": "john@example.com",
  "phone": "+15550001111",
  "first_name": "John",
  "last_name": "Doe",
  "birthday": "1990-01-01T00:00:00Z",
  "device_uuid": "optional-device-uuid"
}
```
Required: `username`, `password`, `email`, `first_name`, `last_name`, `birthday`.

**Response `201`** — `AuthEnvelope` (same shape as login)

**Errors:** `409 username already taken`, `409 email already registered`

---

### `GET /v1/users` — List users
🔒 Requires auth.

**Query params**

| Param     | Default | Description              |
|-----------|---------|--------------------------|
| `page`    | 1       | Page number (1-indexed)  |
| `per_page`| 50      | Items per page           |

**Response `200`** — `PaginatedUsersEnvelope`

---

### `GET /v1/users/{id}` — Get user
🔒 Requires auth.

**Response `200`**
```json
{
  "id": "uuid",
  "username": "johndoe",
  "email": "john@example.com",
  "phone": null,
  "role_id": "uuid",
  "first_name": "John",
  "last_name": "Doe",
  "birthday": "1990-01-01T00:00:00Z",
  "verified": false,
  "email_confirmed": false,
  "phone_confirmed": false,
  "enable": true,
  "created": "2024-01-01T00:00:00Z",
  "updated": "2024-01-01T00:00:00Z"
}
```

---

### `PUT /v1/users/{id}` — Update user
🔒 Requires auth. All fields are optional (partial update).

**Request body**
```json
{
  "username": "newname",
  "email": "new@example.com",
  "phone": "+15550002222",
  "first_name": "Jane",
  "last_name": "Doe",
  "birthday": "1991-06-15T00:00:00Z",
  "role_id": "uuid",
  "enable": true
}
```

**Response `200`** — updated user object

---

### `DELETE /v1/users/{id}` — Delete user
🔒 Requires auth. Soft-deletes the user and all their sessions.

**Response `200`** — `MessageEnvelope`

---

## Password Recovery

### `POST /v1/password-recovery/request` — Request OTP
No auth required. Sends a 6-digit OTP via email or SMS. OTP expires in 15 minutes.

**Request body**
```json
{ "email": "john@example.com" }
```
Or use `"phone_number"` instead of `"email"`.

**Response `200`** — `MessageEnvelope`

---

### `POST /v1/password-recovery/validate-code` — Validate OTP
No auth required. Validates the OTP and returns a new session.

**Request body**
```json
{
  "email": "john@example.com",
  "otp": "123456",
  "device_uuid": "optional-device-uuid"
}
```

**Response `200`** — `AuthEnvelope`

**Errors:** `401 invalid or expired OTP`

---

### `POST /v1/password-recovery/change-password` — Change password
🔒 Requires auth (use the token from validate-code above).

**Request body**
```json
{ "new_password": "newSecret123" }
```

**Response `200`** — `MessageEnvelope`

---

## Email Confirmation

### `POST /v1/confirm-email/request`
🔒 Requires auth. Sends an email confirmation token to the user's registered email.

**Response `200`** — `MessageEnvelope`

---

### `POST /v1/confirm-email/validate-code`
🔒 Requires auth. Validates the token from the email.

**Request body**
```json
{ "token": "32-char-token-from-email" }
```

**Response `200`** — `MessageEnvelope`

---

## Roles

### `GET /v1/roles` — List roles
🔒 Requires auth.

**Response `200`**
```json
[
  { "id": "uuid", "name": "admin", "enable": true, "role_access": ["users", "roles"] }
]
```

---

### `POST /v1/roles` — Create role
🔒 Requires auth.

**Request body**
```json
{ "name": "editor", "enable": true }
```

**Response `201`** — role object

---

### `GET /v1/roles/{id}` — Get role
🔒 Requires auth.

---

### `PUT /v1/roles/{id}` — Update role
🔒 Requires auth.

**Request body** — same as create (all fields optional).

---

### `DELETE /v1/roles/{id}` — Delete role
🔒 Requires auth. Soft-deletes (sets `enable=false`).

---

## Statuses

### `GET /v1/statuses` — List statuses
🔒 Requires auth.

---

### `POST /v1/statuses` — Create status
🔒 Requires auth.

**Request body**
```json
{ "description": "Active" }
```

**Response `201`** — status object

---

### `GET /v1/statuses/{id}` — Get status
🔒 Requires auth.

---

### `PUT /v1/statuses/{id}` — Update status
🔒 Requires auth.

---

### `DELETE /v1/statuses/{id}` — Delete status
🔒 Requires auth. **Hard delete** — permanently removes the item from DynamoDB.

---

## Devices

### `GET /v1/devices` — List devices
🔒 Requires auth. Returns all enabled devices belonging to the current user.

---

### `GET /v1/devices/{id}` — Get device
🔒 Requires auth.

**Response `200`**
```json
{
  "id": "uuid",
  "uuid": "client-device-uuid",
  "user_id": "uuid",
  "token": null,
  "app_version_id": "uuid",
  "enable": true,
  "created": "...",
  "updated": "..."
}
```

---

### `PUT /v1/devices/{id}` — Update device
🔒 Requires auth. Accepts any JSON object of fields to update.

---

### `DELETE /v1/devices/{id}` — Delete device
🔒 Requires auth. Soft-deletes the device.

---

### `PUT /v1/devices/version` — Check app version
🔒 Requires auth. Compares the submitted version against the latest active `app_version` record.

**Request body**
```json
{ "device_version": 2.1 }
```

**Response `200`** — version is up to date  
**Response `409`** — update required

---

## Notifications

### `GET /v1/notifications` — List unread notifications
🔒 Requires auth. Returns unread notifications for the current user, ordered by `created_at` descending.

**Response `200`**
```json
[
  {
    "id": "uuid",
    "user_id": "uuid",
    "device_id": "uuid",
    "template_id": "uuid",
    "message": "You have a new message",
    "readed": 0,
    "created": "...",
    "updated": "..."
  }
]
```

---

### `GET /v1/notifications/{id}` — Get notification
🔒 Requires auth.

---

### `PUT /v1/notifications/{id}` — Mark as read
🔒 Requires auth. Sets `readed = 1`.

**Request body**
```json
{ "read": 1 }
```

**Response `200`** — `MessageEnvelope`

---

## Files (S3)

All files are stored in S3. Metadata (id, name, type, size, owner, visibility) is persisted in DynamoDB. Private files are only accessible by their uploader or an admin.

### `POST /v1/files/s3` — Upload file (multipart)
🔒 Requires auth. Use `multipart/form-data`.

**Query params**

| Param       | Values         | Description                        |
|-------------|----------------|------------------------------------|
| `private`   | `True` / `False` | Mark file as private (default: False) |
| `thumbnail` | `True` / `False` | Mark file as a thumbnail           |

**Response `201`** — file metadata object

---

### `POST /v1/files/s3/base64` — Upload file (base64)
🔒 Requires auth.

**Request body**
```json
{
  "file_name": "photo.jpg",
  "base64": "<base64-encoded-content>"
}
```

**Response `201`** — file metadata object

---

### `GET /v1/files/s3/{id}` — Download file
🔒 Requires auth. Streams the raw file bytes.  
Returns `403` if the file is private and the requester is not the owner.

---

### `GET /v1/files/s3/base64/{id}` — Get file as base64
🔒 Requires auth. Returns the file metadata plus base64-encoded content.

**Response `200`**
```json
{
  "id": "uuid",
  "name": "photo.jpg",
  "type": "image/jpeg",
  "size": 204800,
  "is_private": false,
  "is_thumbnail": 0,
  "object": "files/user-uuid/photo.jpg",
  "base64": "<base64-encoded-content>",
  "user_who_uploaded_id": "uuid",
  "enable": true,
  "created": "...",
  "updated": "..."
}
```

---

### `DELETE /v1/files/s3/{id}` — Delete file
🔒 Requires auth. Deletes from S3 and soft-deletes the metadata record.  
Returns `403` if the requester is not the owner or admin.

---

## Data Model

All IDs are **UUID strings**. Timestamps are **ISO 8601 / RFC 3339** strings.

### DynamoDB tables

| Table               | PK              | SK      | GSIs                                            | Notes                        |
|---------------------|-----------------|---------|------------------------------------------------|------------------------------|
| `users`             | `user_id`       | —       | `username-index`, `email-index`                | Soft-delete via `enable`     |
| `sessions`          | `session_id`    | —       | `user_id-index`                                | Soft-delete via `enable`     |
| `roles`             | `role_id`       | —       | —                                              | Soft-delete via `enable`     |
| `statuses`          | `status_id`     | —       | —                                              | Hard delete                  |
| `devices`           | `device_id`     | —       | `user_id-index`, `device_uuid-index`           | Soft-delete via `enable`     |
| `notifications`     | `notification_id` | —     | `user_id-created_at-index`                     |                              |
| `files`             | `file_id`       | —       | `uploaded_by_user_id-index`                    | Soft-delete via `enable`     |
| `user_verifications`| `user_id`       | `type`  | —                                              | TTL on `expires_at` (15 min) |
| `app_versions`      | `version_id`    | —       | —                                              |                              |

---

## Local Development

### Prerequisites
- Docker (for LocalStack)
- Go 1.23+
- RSA key pair for JWT

### 1. Start LocalStack

```bash
cd infra/localstack
docker-compose up -d
```

LocalStack exposes DynamoDB and S3 at `http://localhost:4566`.  
Tables and the S3 bucket are created automatically at server startup via `dynamo.Bootstrap()`.

### 2. Generate JWT keys

```bash
openssl genrsa -out private_key.pem 2048
openssl rsa -in private_key.pem -pubout -out public_key.pem
```

### 3. Configure environment

```bash
cp .env.example .env
# Edit .env as needed
```

Key variables:

| Variable              | Default               | Description                          |
|-----------------------|-----------------------|--------------------------------------|
| `APP_PORT`            | `3000`                | HTTP server port                     |
| `APP_ENV`             | `development`         | Environment tag                      |
| `AWS_ENDPOINT_URL`    | _(empty = AWS prod)_  | Set to `http://localhost:4566` for LocalStack |
| `AWS_REGION`          | `us-east-1`           | AWS region                           |
| `AWS_ACCESS_KEY_ID`   | _(empty)_             | AWS / LocalStack access key          |
| `AWS_SECRET_ACCESS_KEY` | _(empty)_           | AWS / LocalStack secret key          |
| `S3_BUCKET_NAME`      | `go-api-files`        | S3 bucket for file uploads           |
| `JWT_PRIVATE_KEY_PATH`| `./private_key.pem`   | Path to RSA private key              |
| `JWT_PUBLIC_KEY_PATH` | `./public_key.pem`    | Path to RSA public key               |
| `JWT_EXPIRY_DAYS`     | `7`                   | Token lifetime in days               |
| `SMTP_HOST`           | `localhost`           | SMTP server host                     |
| `SMTP_PORT`           | `1025`                | SMTP server port                     |
| `SMTP_FROM`           | `noreply@example.com` | Sender address                       |

### 4. Run the server

```bash
go run ./cmd/api
```

### Quick smoke test

```bash
# Register
curl -s -X POST http://localhost:3000/v1/users \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"pass","email":"alice@example.com","first_name":"Alice","last_name":"Smith","birthday":"1990-01-01T00:00:00Z"}' | jq .

# Login
TOKEN=$(curl -s -X POST http://localhost:3000/v1/sessions/login \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"pass"}' | jq -r '.Bearer')

# Get current session
curl -s http://localhost:3000/v1/sessions \
  -H "Authorization: Bearer $TOKEN" | jq .
```

---

## Project Structure

```
cmd/api/main.go                   # Entry point — wires all deps, starts server
internal/
  config/                         # Environment-based configuration
  domain/                         # Pure domain structs (User, Session, Role, ...)
  application/
    user/ session/ role/ ...      # Service interfaces + implementations
  infrastructure/
    dynamo/                       # DynamoDB client, bootstrap, repositories
    s3/                           # S3 file store
    jwt/                          # RS256 JWT provider
    smtp/                         # SMTP mailer
    sns/                          # AWS SNS SMS sender
  transport/http/
    handler/                      # HTTP handlers (thin — decode → service → encode)
    middleware/                   # Auth (JWT) and role middleware
    router.go                     # chi router — public vs. authenticated route groups
infra/localstack/
  docker-compose.yml              # LocalStack 3 container
  init-aws.sh                     # Convenience script to pre-create tables via awslocal
```
