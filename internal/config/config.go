package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	AppPort  string
	AppEnv   string
	AWSRegion       string
	AWSEndpointURL  string // empty in prod, set to LocalStack URL in dev
	AWSAccessKeyID  string
	AWSSecretKey    string
	DynamoTables    DynamoTables
	S3BucketName    string
	JWTPrivateKeyPath string
	JWTPublicKeyPath  string
	JWTExpiryDays    int
	RefreshTokenExpiryDays int
	SMTPHost     string
	SMTPPort     string
	SMTPFrom     string
	SMTPUsername string
	SMTPPassword string
	SNSRegion      string
	AllowedOrigins []string // CORS allowed origins
}

// DynamoTables holds the DynamoDB table name for each entity.
type DynamoTables struct {
	Users             string
	Sessions          string
	Statuses          string
	Devices           string
	Notifications     string
	Files             string
	UserVerifications string
	AppVersions       string
}

// Load reads all configuration from environment variables.
func Load() *Config {
	return &Config{
		AppPort: getEnv("APP_PORT", "3000"),
		AppEnv:  getEnv("APP_ENV", "development"),
		AWSRegion:      getEnv("AWS_REGION", "us-east-1"),
		AWSEndpointURL: getEnv("AWS_ENDPOINT_URL", ""),
		AWSAccessKeyID: getEnv("AWS_ACCESS_KEY_ID", ""),
		AWSSecretKey:   getEnv("AWS_SECRET_ACCESS_KEY", ""),
		DynamoTables: DynamoTables{
			Users:             getEnv("DYNAMO_TABLE_USERS", "users"),
			Sessions:          getEnv("DYNAMO_TABLE_SESSIONS", "sessions"),
			Statuses:          getEnv("DYNAMO_TABLE_STATUSES", "statuses"),
			Devices:           getEnv("DYNAMO_TABLE_DEVICES", "devices"),
			Notifications:     getEnv("DYNAMO_TABLE_NOTIFICATIONS", "notifications"),
			Files:             getEnv("DYNAMO_TABLE_FILES", "files"),
			UserVerifications: getEnv("DYNAMO_TABLE_USER_VERIFICATIONS", "user_verifications"),
			AppVersions:       getEnv("DYNAMO_TABLE_APP_VERSIONS", "app_versions"),
		},
		S3BucketName:      getEnv("S3_BUCKET_NAME", "go-api-files"),
		JWTPrivateKeyPath: getEnv("JWT_PRIVATE_KEY_PATH", "./private_key.pem"),
		JWTPublicKeyPath:  getEnv("JWT_PUBLIC_KEY_PATH", "./public_key.pem"),
		JWTExpiryDays:          getEnvInt("JWT_EXPIRY_DAYS", 7),
		RefreshTokenExpiryDays: getEnvInt("REFRESH_TOKEN_EXPIRY_DAYS", 30),
		SMTPHost:     getEnv("SMTP_HOST", "localhost"),
		SMTPPort:     getEnv("SMTP_PORT", "1025"),
		SMTPFrom:     getEnv("SMTP_FROM", "noreply@example.com"),
		SMTPUsername: getEnv("SMTP_USERNAME", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SNSRegion: getEnv("SNS_REGION", "us-east-1"),
		AllowedOrigins: strings.Split(getEnv("ALLOWED_ORIGINS", "*"), ","),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
