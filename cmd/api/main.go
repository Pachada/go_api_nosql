package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-api-nosql/internal/config"
	"github.com/go-api-nosql/internal/infrastructure/dynamo"
	jwtinfra "github.com/go-api-nosql/internal/infrastructure/jwt"
	s3infra "github.com/go-api-nosql/internal/infrastructure/s3"
	"github.com/go-api-nosql/internal/infrastructure/smtp"
	"github.com/go-api-nosql/internal/infrastructure/sns"
	transporthttp "github.com/go-api-nosql/internal/transport/http"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment")
	}

	cfg := config.Load()

	// Bootstrap DynamoDB tables (creates them if they don't exist).
	dynamoClient := dynamo.NewClient(cfg)
	dynamo.Bootstrap(context.Background(), dynamoClient, cfg.DynamoTables)

	// JWT provider (optional — graceful fallback if keys are missing).
	var jwtProvider *jwtinfra.Provider
	if p, err := jwtinfra.NewProvider(cfg); err == nil {
		jwtProvider = p
	} else {
		log.Printf("WARN: JWT provider not available: %v", err)
	}

	// S3 store.
	s3Client := s3infra.NewClient(cfg)
	s3Store := s3infra.NewStore(s3Client, cfg.S3BucketName)

	// SMTP mailer.
	mailer := smtp.NewMailer(cfg)

	// SNS SMS sender (optional — graceful fallback).
	var smsSender sns.SMSSender
	if sender, err := sns.NewSender(cfg); err == nil {
		smsSender = sender
	} else {
		log.Printf("WARN: SNS sender not available: %v", err)
	}

	deps := &transporthttp.Deps{
		UserRepo:         dynamo.NewUserRepo(dynamoClient, cfg.DynamoTables.Users),
		SessionRepo:      dynamo.NewSessionRepo(dynamoClient, cfg.DynamoTables.Sessions),
		StatusRepo:       dynamo.NewStatusRepo(dynamoClient, cfg.DynamoTables.Statuses),
		DeviceRepo:       dynamo.NewDeviceRepo(dynamoClient, cfg.DynamoTables.Devices),
		NotificationRepo: dynamo.NewNotificationRepo(dynamoClient, cfg.DynamoTables.Notifications),
		FileRepo:         dynamo.NewFileRepo(dynamoClient, cfg.DynamoTables.Files),
		VerificationRepo: dynamo.NewVerificationRepo(dynamoClient, cfg.DynamoTables.UserVerifications),
		AppVersionRepo:   dynamo.NewAppVersionRepo(dynamoClient, cfg.DynamoTables.AppVersions),
		S3Store:          s3Store,
		Mailer:           mailer,
		SMSSender:        smsSender,
		JWTProvider:      jwtProvider,
	}

	router := transporthttp.NewRouter(context.Background(), cfg, deps)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.AppPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server starting on :%s (env=%s)", cfg.AppPort, cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
	log.Println("Server stopped")
}
