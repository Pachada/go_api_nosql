package http

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	dynamodbsdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/go-api-nosql/internal/application/auth"
	"github.com/go-api-nosql/internal/application/device"
	fileapp "github.com/go-api-nosql/internal/application/file"
	"github.com/go-api-nosql/internal/application/notification"
	"github.com/go-api-nosql/internal/application/session"
	"github.com/go-api-nosql/internal/application/status"
	"github.com/go-api-nosql/internal/application/user"
	"github.com/go-api-nosql/internal/config"
	"github.com/go-api-nosql/internal/domain"
	googleinfra "github.com/go-api-nosql/internal/infrastructure/google"
	jwtinfra "github.com/go-api-nosql/internal/infrastructure/jwt"
	"github.com/go-api-nosql/internal/infrastructure/smtp"
	"github.com/go-api-nosql/internal/infrastructure/sns"
	"github.com/go-api-nosql/internal/transport/http/handler"
	appmiddleware "github.com/go-api-nosql/internal/transport/http/middleware"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"golang.org/x/time/rate"
)

// Deps holds all infrastructure dependencies for the router.
type Deps struct {
	UserRepo         UserRepository
	SessionRepo      SessionRepository
	StatusRepo       StatusRepository
	DeviceRepo       DeviceRepository
	NotificationRepo NotificationRepository
	FileRepo         FileRepository
	VerificationRepo VerificationRepository
	AppVersionRepo   AppVersionRepository
	DynamoClient     *dynamodbsdk.Client
	S3Store          ObjectStore
	Mailer           smtp.Mailer
	SMSSender        sns.SMSSender
	JWTProvider      *jwtinfra.Provider
}

// dynamoPinger adapts *dynamodb.Client to the handler.dbPinger interface.
type dynamoPinger struct{ client *dynamodbsdk.Client }

func (p *dynamoPinger) Ping(ctx context.Context) error {
	_, err := p.client.ListTables(ctx, &dynamodbsdk.ListTablesInput{Limit: aws.Int32(1)})
	return err
}

// googleVerifierAdapter adapts *googleinfra.Verifier to session.googleVerifier.
type googleVerifierAdapter struct{ v *googleinfra.Verifier }

func (a *googleVerifierAdapter) Verify(ctx context.Context, token string) (*session.GooglePayload, error) {
	p, err := a.v.Verify(ctx, token)
	if err != nil {
		return nil, err
	}
	return &session.GooglePayload{
		Sub:           p.Sub,
		Email:         p.Email,
		EmailVerified: p.EmailVerified,
		FirstName:     p.FirstName,
		LastName:      p.LastName,
	}, nil
}

// NewRouter builds and returns the application router.
func NewRouter(ctx context.Context, cfg *config.Config, deps *Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(appmiddleware.RequestLogger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false, // Bearer token auth; cookies not used
		MaxAge:           300,
	}))

	if deps.JWTProvider == nil {
		log.Fatal("JWT provider is required but was not initialized; check RSA key files")
	}
	authMw := appmiddleware.Auth(deps.JWTProvider)

	// 5 requests/second, burst of 10 — applied to sensitive public endpoints.
	sensitiveRL := appmiddleware.NewRateLimiter(ctx, rate.Limit(5), 10)

	refreshDur := time.Duration(cfg.RefreshTokenExpiryDays) * 24 * time.Hour
	sessionSvc := session.NewService(session.ServiceDeps{
		SessionRepo:     deps.SessionRepo,
		UserRepo:        deps.UserRepo,
		DeviceRepo:      deps.DeviceRepo,
		JWTProvider:     deps.JWTProvider,
		GoogleVerifier:  &googleVerifierAdapter{v: googleinfra.NewVerifier(cfg.GoogleClientID)},
		RefreshTokenDur: refreshDur,
	})
	userSvc := user.NewService(user.ServiceDeps{
		UserRepo:        deps.UserRepo,
		SessionRepo:     deps.SessionRepo,
		DeviceRepo:      deps.DeviceRepo,
		JWTProvider:     deps.JWTProvider,
		RefreshTokenDur: refreshDur,
	})
	statusSvc := status.NewService(deps.StatusRepo)
	deviceSvc := device.NewService(deps.DeviceRepo, deps.AppVersionRepo)
	notifSvc := notification.NewService(deps.NotificationRepo)
	fileSvc := fileapp.NewService(deps.S3Store, deps.FileRepo)
	authSvc := auth.NewService(auth.ServiceDeps{
		VerificationRepo: deps.VerificationRepo,
		UserRepo:         deps.UserRepo,
		SessionRepo:      deps.SessionRepo,
		DeviceRepo:       deps.DeviceRepo,
		Mailer:           deps.Mailer,
		SMSSender:        deps.SMSSender,
		JWTProvider:      deps.JWTProvider,
		RefreshTokenDur:  refreshDur,
	})

	healthH := handler.NewHealthHandler(&dynamoPinger{deps.DynamoClient})
	sessionH := handler.NewSessionHandler(sessionSvc)
	userH := handler.NewUserHandler(userSvc)
	statusH := handler.NewStatusHandler(statusSvc)
	deviceH := handler.NewDeviceHandler(deviceSvc)
	notifH := handler.NewNotificationHandler(notifSvc)
	fileH := handler.NewFileHandler(fileSvc)
	pwH := handler.NewPasswordRecoveryHandler(authSvc)
	emailH := handler.NewEmailConfirmHandler(authSvc)
	phoneH := handler.NewPhoneConfirmHandler(authSvc)

	r.Route("/v1", func(r chi.Router) {
		// ── Public routes (no auth) ──────────────────────────────────────────
		r.Get("/health-check/{action}", healthH.Ping)
		r.Post("/health-check/{action}", healthH.Ping)
		r.Get("/roles", handler.ListRoles)
		r.With(sensitiveRL.Limit).Post("/sessions/login", sessionH.Login)
		r.With(sensitiveRL.Limit).Post("/sessions/google", sessionH.GoogleLogin)
		r.Post("/sessions/refresh", sessionH.Refresh)
		r.With(sensitiveRL.Limit).Post("/users", userH.Register)
		r.With(sensitiveRL.Limit).Post("/password-recovery/{action}", pwH.Action)

		// ── Authenticated routes ─────────────────────────────────────────────
		r.Group(func(r chi.Router) {
			r.Use(authMw)

			r.Get("/sessions", sessionH.GetCurrent)
			r.Post("/sessions/logout", sessionH.Logout)

			// Any authenticated user
			r.Get("/users/{id}", userH.Get)
			r.Put("/users/{id}", userH.Update)
			r.Post("/users/me/password", userH.ChangePassword)
			r.Get("/statuses", statusH.List)
			r.Get("/statuses/{id}", statusH.Get)
			r.Get("/devices", deviceH.List)
			r.Put("/devices/version", deviceH.CheckVersion)
			r.Get("/devices/{id}", deviceH.Get)
			r.Put("/devices/{id}", deviceH.Update)
			r.Delete("/devices/{id}", deviceH.Delete)
			r.Get("/notifications", notifH.ListUnread)
			r.Put("/notifications/{id}", notifH.MarkAsRead)
			r.Post("/files/s3", fileH.Upload)
			r.Post("/files/s3/base64", fileH.UploadBase64)
			r.Get("/files/s3/base64/{id}", fileH.GetBase64)
			r.Get("/files/s3/{id}", fileH.Download)
			r.Delete("/files/s3/{id}", fileH.Delete)
			r.With(sensitiveRL.Limit).Post("/confirm-email/{action}", emailH.Action)
			r.With(sensitiveRL.Limit).Post("/confirm-phone/{action}", phoneH.Action)

			// Admin-only routes
			r.Group(func(r chi.Router) {
				r.Use(appmiddleware.RequireRole(domain.RoleAdmin))

				r.Get("/users", userH.List)
				r.Delete("/users/{id}", userH.Delete)

				r.Post("/statuses", statusH.Create)
				r.Put("/statuses/{id}", statusH.Update)
				r.Delete("/statuses/{id}", statusH.Delete)
			})
		})
	})

	return r
}
