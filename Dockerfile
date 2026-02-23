# Stage 1: Build the Go binary.
FROM golang:1.24-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o server ./cmd/api

# Stage 2: Lambda-compatible production image.
# Uses the AWS-provided Lambda runtime base image and the Lambda Web Adapter,
# which transparently proxies Lambda invocations to the existing HTTP server —
# no changes to main.go required.
FROM public.ecr.aws/lambda/provided:al2023

# Lambda Web Adapter forwards API Gateway payloads to the HTTP server.
COPY --from=public.ecr.aws/awsguru/aws-lambda-adapter:latest \
    /lambda-adapter /opt/extensions/lambda-adapter

COPY --from=builder /build/server /var/task/server

# Port the HTTP server listens on; Lambda Web Adapter reads AWS_LWA_PORT.
ENV PORT=3000
ENV AWS_LWA_PORT=3000

CMD ["/var/task/server"]
