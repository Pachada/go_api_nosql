package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-api-nosql/internal/application/auth"
	"github.com/go-api-nosql/internal/application/device"
	fileapp "github.com/go-api-nosql/internal/application/file"
	"github.com/go-api-nosql/internal/application/notification"
	"github.com/go-api-nosql/internal/application/session"
	"github.com/go-api-nosql/internal/application/status"
	"github.com/go-api-nosql/internal/application/user"
	"github.com/go-api-nosql/internal/config"
	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/infrastructure/dynamo"
	jwtinfra "github.com/go-api-nosql/internal/infrastructure/jwt"
	s3infra "github.com/go-api-nosql/internal/infrastructure/s3"
	"github.com/go-api-nosql/internal/infrastructure/smtp"
	"github.com/go-api-nosql/internal/infrastructure/sns"
	"github.com/go-api-nosql/internal/transport/http/handler"
	appmiddleware "github.com/go-api-nosql/internal/transport/http/middleware"
	"golang.org/x/time/rate"
)

// Deps holds all infrastructure dependencies for the router.
type Deps struct {
	UserRepo         *dynamo.UserRepo
	SessionRepo      *dynamo.SessionRepo
	StatusRepo       *dynamo.StatusRepo
	DeviceRepo       *dynamo.DeviceRepo
	NotificationRepo *dynamo.NotificationRepo
	FileRepo         *dynamo.FileRepo
	VerificationRepo *dynamo.VerificationRepo
	AppVersionRepo   *dynamo.AppVersionRepo
	S3Store          *s3infra.Store
	Mailer           smtp.Mailer
	SMSSender        sns.SMSSender
	JWTProvider      *jwtinfra.Provider
}

// NewRouter builds and returns the application router.
func NewRouter(cfg *config.Config, deps *Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	var authMw func(http.Handler) http.Handler
	if deps.JWTProvider != nil {
		authMw = appmiddleware.Auth(deps.JWTProvider)
	} else {
		authMw = func(next http.Handler) http.Handler { return next }
	}

	// 5 requests/second, burst of 10 — applied to sensitive public endpoints.
	sensitiveRL := appmiddleware.NewRateLimiter(rate.Limit(5), 10)

	sessionSvc := session.NewService(deps.SessionRepo, deps.UserRepo, deps.DeviceRepo, deps.JWTProvider)
	userSvc := user.NewService(deps.UserRepo, deps.SessionRepo, deps.DeviceRepo, deps.JWTProvider)
	statusSvc := status.NewService(deps.StatusRepo)
	deviceSvc := device.NewService(deps.DeviceRepo, deps.AppVersionRepo)
	notifSvc := notification.NewService(deps.NotificationRepo)
	fileSvc := fileapp.NewService(deps.S3Store, deps.FileRepo)
	authSvc := auth.NewService(deps.VerificationRepo, deps.UserRepo, deps.SessionRepo, deps.DeviceRepo, deps.Mailer, deps.SMSSender, deps.JWTProvider)

	healthH := handler.NewHealthHandler()
	sessionH := handler.NewSessionHandler(sessionSvc)
	userH := handler.NewUserHandler(userSvc)
	statusH := handler.NewStatusHandler(statusSvc)
	deviceH := handler.NewDeviceHandler(deviceSvc)
	notifH := handler.NewNotificationHandler(notifSvc)
	fileH := handler.NewFileHandler(fileSvc)
	pwH := handler.NewPasswordRecoveryHandler(authSvc)
	emailH := handler.NewEmailConfirmHandler(authSvc)

	r.Route("/v1", func(r chi.Router) {
		// ── Public routes (no auth) ──────────────────────────────────────────
		r.Get("/health-check/{action}", healthH.Ping)
		r.Post("/health-check/{action}", healthH.Ping)
		r.Get("/test", healthH.Test)
		r.Post("/test", healthH.Test)
		r.With(sensitiveRL.Limit).Post("/sessions/login", sessionH.Login)
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
			r.Get("/roles", handler.ListRoles)
			r.Get("/statuses", statusH.List)
			r.Get("/statuses/{id}", statusH.Get)
			r.Get("/devices", deviceH.List)
			r.Put("/devices/version", deviceH.CheckVersion)
			r.Get("/devices/{id}", deviceH.Get)
			r.Put("/devices/{id}", deviceH.Update)
			r.Delete("/devices/{id}", deviceH.Delete)
			r.Get("/notifications", notifH.ListUnread)
			r.Get("/notifications/{id}", notifH.Get)
			r.Put("/notifications/{id}", notifH.MarkAsRead)
			r.Post("/files/s3", fileH.Upload)
			r.Get("/files/s3/base64", fileH.ListBase64)
			r.Post("/files/s3/base64", fileH.UploadBase64)
			r.Get("/files/s3/base64/{id}", fileH.GetBase64)
			r.Post("/files/s3/base64/{id}", fileH.MethodNotAllowed)
			r.Get("/files/s3/{id}", fileH.Download)
			r.Delete("/files/s3/{id}", fileH.Delete)
			r.Post("/password-recovery/change-password", pwH.ChangePassword)
			r.Post("/confirm-email/{action}", emailH.Action)

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
