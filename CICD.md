# CI/CD Configuration Guide

This document explains how to configure the GitHub Actions pipeline and AWS infrastructure to enable automated linting, testing, and Docker image delivery to Amazon ECR.

---

## Overview

The pipeline (`.github/workflows/ci.yml`) runs three sequential jobs on every push or pull request:

```
lint → test → build-and-push (push events only)
```

| Job | Trigger | What it does |
|---|---|---|
| **lint** | push + PR | Runs `golangci-lint` against `.golangci.yml` |
| **test** | push + PR | Runs `go test -race ./...` |
| **build-and-push** | push to `main`/`develop` only | Builds the Docker image and pushes to ECR |

---

## 1. AWS Setup

### 1.1 Create an ECR Repository

```bash
aws ecr create-repository \
  --repository-name <your-repo-name> \
  --region <your-region>
```

Note the repository name — you will need it in [step 3](#3-github-repository-configuration).

### 1.2 Configure GitHub OIDC Identity Provider in AWS

The pipeline authenticates to AWS using OpenID Connect (OIDC) — no long-lived access keys are stored in GitHub.

1. Open the [IAM console](https://console.aws.amazon.com/iam/) → **Identity providers** → **Add provider**.
2. Select **OpenID Connect** and fill in:
   - **Provider URL**: `https://token.actions.githubusercontent.com`
   - **Audience**: `sts.amazonaws.com`
3. Click **Add provider**.

> This only needs to be done **once per AWS account**.

### 1.3 Create an IAM Role for GitHub Actions

Create a role that GitHub Actions will assume to push images to ECR.

**Trust policy** (`trust-policy.json`) — replace the placeholders:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::<ACCOUNT_ID>:oidc-provider/token.actions.githubusercontent.com"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "token.actions.githubusercontent.com:aud": "sts.amazonaws.com"
        },
        "StringLike": {
          "token.actions.githubusercontent.com:sub": "repo:<GITHUB_ORG_OR_USER>/<GITHUB_REPO>:*"
        }
      }
    }
  ]
}
```

**Permissions policy** — attach the AWS managed policy `AmazonEC2ContainerRegistryPowerUser`, or use this minimal inline policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ecr:GetAuthorizationToken"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "ecr:BatchCheckLayerAvailability",
        "ecr:CompleteLayerUpload",
        "ecr:InitiateLayerUpload",
        "ecr:PutImage",
        "ecr:UploadLayerPart",
        "ecr:BatchGetImage",
        "ecr:GetDownloadUrlForLayer"
      ],
      "Resource": "arn:aws:ecr:<REGION>:<ACCOUNT_ID>:repository/<ECR_REPOSITORY_NAME>"
    }
  ]
}
```

Create the role:

```bash
aws iam create-role \
  --role-name github-actions-ecr \
  --assume-role-policy-document file://trust-policy.json

aws iam put-role-policy \
  --role-name github-actions-ecr \
  --policy-name ECRPush \
  --policy-document file://ecr-policy.json
```

Note the **Role ARN** (e.g. `arn:aws:iam::123456789012:role/github-actions-ecr`).

---

## 2. Docker Image Notes

The `Dockerfile` uses a two-stage build:

1. **Builder stage** (`golang:1.24-alpine`) — compiles a static binary from `./cmd/api`.
2. **Production stage** (`public.ecr.aws/lambda/provided:al2023`) — minimal Lambda-compatible image that includes the [AWS Lambda Web Adapter](https://github.com/awslabs/aws-lambda-web-adapter).

The Lambda Web Adapter allows the existing `http.Server` in `main.go` to run on AWS Lambda without any code changes. It intercepts Lambda invocations from API Gateway and forwards them as HTTP requests to the application on port `3000`.

### JWT PEM Keys

`main.go` reads RSA keys from the filesystem (`JWT_PRIVATE_KEY_PATH`, `JWT_PUBLIC_KEY_PATH`). These files are excluded from the Docker image via `.dockerignore`. Before deploying to Lambda you must:

- Store the key contents in **AWS Secrets Manager** or **SSM Parameter Store**.
- At container startup (e.g. via an entrypoint script), fetch the values and write them to `/tmp/private_key.pem` and `/tmp/public_key.pem`.
- Set the environment variables to point to those paths.

---

## 3. GitHub Repository Configuration

Go to your repository → **Settings** → **Secrets and variables** → **Actions**.

### Variables (not secret, visible in logs)

| Name | Example value | Description |
|---|---|---|
| `AWS_REGION` | `us-east-1` | AWS region where ECR lives |
| `ECR_REPOSITORY` | `go-api-nosql` | ECR repository **name** (not the full URI) |

### Secrets (encrypted, never shown in logs)

| Name | Example value | Description |
|---|---|---|
| `AWS_ROLE_TO_ASSUME` | `arn:aws:iam::123456789012:role/github-actions-ecr` | IAM role ARN from step 1.3 |

---

## 4. Image Tagging Strategy

Each successful push to `main` or `develop` produces two tags:

| Tag | Value | Purpose |
|---|---|---|
| Commit SHA | `abc1234...` | Immutable — safe to reference in deployments |
| `latest` | always updated | Convenience tag for manual testing |

When deploying to Lambda, reference the **SHA tag** for reproducible deployments:

```bash
aws lambda update-function-code \
  --function-name <your-function> \
  --image-uri <ACCOUNT_ID>.dkr.ecr.<REGION>.amazonaws.com/<ECR_REPOSITORY>:<SHA>
```

---

## 5. Lambda Environment Variables

Set these on the Lambda function under **Configuration → Environment variables**.

> **AWS credentials** (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_ENDPOINT_URL`) must **not** be set on Lambda — the function uses its **execution role** automatically.

### Application

| Variable | Required | Example | Notes |
|---|---|---|---|
| `APP_PORT` | yes | `3000` | Must match `PORT` / `AWS_LWA_PORT` in the Dockerfile |
| `APP_ENV` | yes | `production` | Controls behaviour flags inside the app |
| `ALLOWED_ORIGINS` | yes | `https://app.example.com` | Comma-separated list of CORS origins; avoid `*` in production |

### DynamoDB Table Names

| Variable | Required | Example |
|---|---|---|
| `DYNAMO_TABLE_USERS` | yes | `users` |
| `DYNAMO_TABLE_SESSIONS` | yes | `sessions` |
| `DYNAMO_TABLE_STATUSES` | yes | `statuses` |
| `DYNAMO_TABLE_DEVICES` | yes | `devices` |
| `DYNAMO_TABLE_NOTIFICATIONS` | yes | `notifications` |
| `DYNAMO_TABLE_FILES` | yes | `files` |
| `DYNAMO_TABLE_USER_VERIFICATIONS` | yes | `user_verifications` |
| `DYNAMO_TABLE_APP_VERSIONS` | yes | `app_versions` |

### S3

| Variable | Required | Example |
|---|---|---|
| `S3_BUCKET_NAME` | yes | `go-api-files` |

### JWT (RS256)

Lambda has a read-only filesystem except for `/tmp`. PEM key files cannot be bundled in the image (they are excluded by `.dockerignore`). The recommended approach is:

1. Store each key in **AWS Secrets Manager** (or SSM Parameter Store SecureString).
2. Add an entrypoint script that fetches them and writes to `/tmp` before starting the server. *(Or refactor `config.go` to read key content directly from env vars.)*

| Variable | Required | Example |
|---|---|---|
| `JWT_PRIVATE_KEY_PATH` | yes | `/tmp/private_key.pem` |
| `JWT_PUBLIC_KEY_PATH` | yes | `/tmp/public_key.pem` |
| `JWT_EXPIRY_DAYS` | no | `7` |
| `REFRESH_TOKEN_EXPIRY_DAYS` | no | `30` |

### SMTP (email)

| Variable | Required | Example |
|---|---|---|
| `SMTP_HOST` | yes | `email-smtp.us-east-1.amazonaws.com` |
| `SMTP_PORT` | yes | `587` |
| `SMTP_FROM` | yes | `noreply@example.com` |
| `SMTP_USERNAME` | yes | *(SES SMTP credential)* |
| `SMTP_PASSWORD` | yes | *(SES SMTP credential — use a secret)* |
| `SMTP_TLS` | yes | `true` |

### SNS (SMS)

| Variable | Required | Example |
|---|---|---|
| `SNS_REGION` | yes | `us-east-1` |

### Lambda Execution Role Permissions

The Lambda execution role must have IAM policies that allow the function to reach the services above. Minimum required actions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:GetItem",
        "dynamodb:PutItem",
        "dynamodb:UpdateItem",
        "dynamodb:DeleteItem",
        "dynamodb:Query",
        "dynamodb:Scan",
        "dynamodb:BatchWriteItem",
        "dynamodb:DescribeTable",
        "dynamodb:CreateTable"
      ],
      "Resource": "arn:aws:dynamodb:<REGION>:<ACCOUNT_ID>:table/*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::<S3_BUCKET_NAME>",
        "arn:aws:s3:::<S3_BUCKET_NAME>/*"
      ]
    },
    {
      "Effect": "Allow",
      "Action": ["sns:Publish"],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue"
      ],
      "Resource": "arn:aws:secretsmanager:<REGION>:<ACCOUNT_ID>:secret:*"
    }
  ]
}
```

---

## 6. Quick-Start Checklist

- [ ] Create ECR repository
- [ ] Add GitHub OIDC provider to AWS account (one-time)
- [ ] Create IAM role with trust policy scoped to this repository
- [ ] Add `AWS_REGION` and `ECR_REPOSITORY` as repository **variables**
- [ ] Add `AWS_ROLE_TO_ASSUME` as a repository **secret**
- [ ] Push to `main` or `develop` and verify the Actions tab
- [ ] Create Lambda function (container image) pointing to the ECR image
- [ ] Attach execution role with DynamoDB, S3, SNS, and Secrets Manager permissions
- [ ] Set all environment variables from section 5 on the Lambda function
- [ ] Store JWT PEM keys in Secrets Manager and wire up the paths
