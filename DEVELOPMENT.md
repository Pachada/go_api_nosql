# Local Development Guide

## Overview

This project is a Go REST API backed by **AWS DynamoDB** (NoSQL) and **Amazon S3**, using [LocalStack](https://localstack.cloud) to emulate both services locally — no real AWS account required.

---

## Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Go | 1.24+ | `go version` |
| Docker + Docker Compose | any recent | For LocalStack |
| `openssl` | any | To generate RSA keys |

---

## 1. Clone & configure

```bash
cp .env.example .env
```

The defaults in `.env.example` are already wired to LocalStack, so no changes are needed for a first run.

---

## 2. Generate RSA keys (JWT)

The API signs JWTs with RS256. Generate a keypair once:

```bash
openssl genrsa -out private_key.pem 2048
openssl rsa -in private_key.pem -pubout -out public_key.pem
```

Place both files at the project root (default paths used by `.env.example`).

---

## 3. Start LocalStack

LocalStack emulates DynamoDB, S3, and SNS locally. Start it from the `infra/localstack` directory:

```bash
cd infra/localstack
docker compose up -d
```

Wait for the health check to pass (usually ~10 seconds):

```bash
docker compose ps          # STATUS should show "healthy"
curl http://localhost:4566/_localstack/health
```

The `init-aws.sh` script runs automatically on first startup and creates all DynamoDB tables and the S3 bucket.

---

## 4. Run the API

### Standard run

```bash
go run ./cmd/api
```

### Hot reload (recommended for development)

Uses [Air](https://github.com/air-verse/air) to watch for file changes and auto-restart:

```bash
air
```

Install Air once with:

```bash
go install github.com/air-verse/air@latest
```

The `.air.toml` at the project root is already configured. The compiled binary goes to `tmp/` (git-ignored).

The server starts on port `3000` by default (set `APP_PORT` in `.env` to change).

```
Server starting on :3000 (env=development)
```

On startup, `dynamo.Bootstrap()` calls `CreateTable` for every table — it silently skips tables that already exist, so it is safe to call on every boot.

---

## 5. Reset LocalStack (wipe all data)

To destroy all data and start completely fresh:

```bash
cd infra/localstack
docker compose down -v   # stops container AND deletes the named volume
docker compose up -d     # fresh container — init-aws.sh runs automatically
```

> **The `-v` flag is required.** Without it the named volume (`localstack_data`) persists and old data survives the restart.

On fresh start, `init-aws.sh` automatically:
- Creates all DynamoDB tables with their GSIs
- Creates the S3 bucket
- Seeds the default roles (`Admin` id=1, `User` id=2)

---

## 6. Verify it works

```bash
curl http://localhost:3000/v1/health-check/ping
```

---

## DynamoDB "Migrations" vs Goose

In a relational project you'd use a migration tool like **Goose** to version SQL schema changes (`ALTER TABLE`, `CREATE INDEX`, etc.). DynamoDB requires a different approach because:

- DynamoDB is **schema-less** — items in the same table can have different attributes; only the key attributes are enforced at the table level.
- You cannot run SQL `ALTER TABLE` statements; structural changes happen through the AWS API.

### How this project handles it

#### Table creation — `Bootstrap()`

`internal/infrastructure/dynamo/bootstrap.go` contains a `Bootstrap()` function that is called once at startup (`main.go`). It issues a `CreateTable` call for every table. DynamoDB returns `ResourceInUseException` if the table already exists — the function ignores that error and moves on.

This replaces the "initial migration" you'd have in Goose.

#### Adding a new GSI (equivalent of `ALTER TABLE … ADD INDEX`)

DynamoDB lets you add a GSI to an existing table live, without downtime, via `UpdateTable`. The code-side changes required are:

1. **Update `bootstrap.go`** — add the new GSI to the `CreateTableInput`. New environments (and LocalStack) will get it automatically.
2. **Add the GSI to the existing table** — run this once against your environment (LocalStack or AWS):

```bash
# LocalStack example
awslocal dynamodb update-table \
  --table-name sessions \
  --attribute-definitions AttributeName=refresh_token,AttributeType=S \
  --global-secondary-index-updates \
    '[{"Create":{"IndexName":"refresh_token-index","KeySchema":[{"AttributeName":"refresh_token","KeyType":"HASH"}],"Projection":{"ProjectionType":"ALL"}}}]'
```

3. **Update `infra/localstack/init-aws.sh`** — so fresh LocalStack containers also get the GSI.

> This is exactly the pattern used when adding the `refresh_token-index` GSI in this project.

#### Changing item shape (equivalent of `ALTER TABLE … ADD COLUMN`)

No action needed at the DynamoDB level. DynamoDB stores only the attributes you write. To add a new field:

1. Add the field to the Go struct with a `dynamodbav:"field_name"` tag.
2. New writes will include it automatically.
3. Existing items simply won't have the attribute until they are re-written — handle this with a nil/zero check in your read logic.

#### Removing a field

Remove it from the Go struct. Existing items retain the attribute in DynamoDB but it will be ignored by `attributevalue.UnmarshalMap`. If you want to scrub it from storage, run a one-off scan + update.

#### Renaming a GSI or changing key types

DynamoDB does **not** support modifying or deleting a GSI's key schema. You must:
1. Create a new GSI with the desired configuration.
2. Backfill / wait for it to become `ACTIVE`.
3. Update application code to use the new index name.
4. Delete the old GSI.

#### Summary table

| SQL (Goose) | DynamoDB equivalent |
|---|---|
| Initial schema (`001_create_tables.sql`) | `Bootstrap()` in `bootstrap.go` |
| `CREATE INDEX` | Add GSI in `bootstrap.go` + run `UpdateTable` once |
| `ALTER TABLE ADD COLUMN` | Add field to Go struct + `dynamodbav` tag |
| `ALTER TABLE DROP COLUMN` | Remove from struct; scrub old data manually if needed |
| `ALTER TABLE RENAME COLUMN` | Add new attr, migrate data, remove old attr |
| Version tracking | Git history of `bootstrap.go` and `init-aws.sh` |

---

## Environment Variables Reference

| Variable | Default | Description |
|---|---|---|
| `APP_PORT` | `3000` | HTTP listen port |
| `APP_ENV` | `development` | Environment label |
| `AWS_ENDPOINT_URL` | *(empty)* | Set to `http://localhost:4566` for LocalStack |
| `AWS_REGION` | `us-east-1` | AWS region |
| `AWS_ACCESS_KEY_ID` | *(empty)* | Use `test` for LocalStack |
| `AWS_SECRET_ACCESS_KEY` | *(empty)* | Use `test` for LocalStack |
| `DYNAMO_TABLE_USERS` | `users` | DynamoDB table name |
| `DYNAMO_TABLE_SESSIONS` | `sessions` | |
| `DYNAMO_TABLE_ROLES` | `roles` | |
| `DYNAMO_TABLE_STATUSES` | `statuses` | |
| `DYNAMO_TABLE_DEVICES` | `devices` | |
| `DYNAMO_TABLE_NOTIFICATIONS` | `notifications` | |
| `DYNAMO_TABLE_FILES` | `files` | |
| `DYNAMO_TABLE_USER_VERIFICATIONS` | `user_verifications` | |
| `DYNAMO_TABLE_APP_VERSIONS` | `app_versions` | |
| `S3_BUCKET_NAME` | `go-api-files` | S3 bucket for file uploads |
| `JWT_PRIVATE_KEY_PATH` | `./private_key.pem` | RS256 private key |
| `JWT_PUBLIC_KEY_PATH` | `./public_key.pem` | RS256 public key |
| `JWT_EXPIRY_DAYS` | `7` | Access token lifetime in days |
| `SMTP_HOST` | `localhost` | |
| `SMTP_PORT` | `1025` | |
| `SMTP_FROM` | `noreply@example.com` | |
| `SMTP_USERNAME` | *(empty)* | |
| `SMTP_PASSWORD` | *(empty)* | |
| `SNS_REGION` | `us-east-1` | AWS region for SMS via SNS |
