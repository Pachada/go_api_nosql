package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-api-nosql/internal/domain"
)

// SafeUser is the public-facing user DTO that omits sensitive fields like PasswordHash.
type SafeUser struct {
	UserID         string    `json:"id"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	Phone          *string   `json:"phone,omitempty"`
	Role           string    `json:"role"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Birthday       string    `json:"birthday,omitempty"`
	Verified       bool      `json:"verified"`
	EmailConfirmed bool      `json:"email_confirmed"`
	PhoneConfirmed bool      `json:"phone_confirmed"`
	Enable         bool      `json:"enable"`
	CreatedAt      time.Time `json:"created"`
	UpdatedAt      time.Time `json:"updated"`
}

// SafeSession is the public-facing session DTO that omits RefreshToken, RefreshExpiresAt, and User.
type SafeSession struct {
	SessionID string    `json:"id"`
	UserID    string    `json:"user_id"`
	DeviceID  *string   `json:"device_id"`
	Enable    bool      `json:"enable"`
	CreatedAt time.Time `json:"created"`
	UpdatedAt time.Time `json:"updated"`
}

func toSafeUser(u *domain.User) *SafeUser {
	if u == nil {
		return nil
	}
	return &SafeUser{
		UserID:         u.UserID,
		Username:       u.Username,
		Email:          u.Email,
		Phone:          u.Phone,
		Role:           u.Role,
		FirstName:      u.FirstName,
		LastName:       u.LastName,
		Birthday:       formatDate(u.Birthday),
		Verified:       u.Verified,
		EmailConfirmed: u.EmailConfirmed,
		PhoneConfirmed: u.PhoneConfirmed,
		Enable:         u.Enable,
		CreatedAt:      u.CreatedAt,
		UpdatedAt:      u.UpdatedAt,
	}
}

func toSafeSession(s *domain.Session) *SafeSession {
	if s == nil {
		return nil
	}
	var deviceID *string
	if s.DeviceID != "" {
		deviceID = &s.DeviceID
	}
	return &SafeSession{
		SessionID: s.SessionID,
		UserID:    s.UserID,
		DeviceID:  deviceID,
		Enable:    s.Enable,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

// MessageEnvelope is the generic response wrapper.
type MessageEnvelope struct {
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
	ErrorCode int    `json:"error_code,omitempty"`
}

// AuthEnvelope wraps login/register responses.
type AuthEnvelope struct {
	AccessToken  string       `json:"access_token,omitempty"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	Session      *SafeSession `json:"session,omitempty"`
	User         *SafeUser    `json:"user,omitempty"`
	Message      string       `json:"message,omitempty"`
	Error        string       `json:"error,omitempty"`
}

// SessionEnvelope wraps current-session responses.
type SessionEnvelope struct {
	Session *SafeSession `json:"session,omitempty"`
	User    *SafeUser    `json:"user,omitempty"`
	Message string       `json:"message,omitempty"`
	Error   string       `json:"error,omitempty"`
}

// PaginatedUsersEnvelope wraps paginated user list responses.
type PaginatedUsersEnvelope struct {
	MaxPage    int         `json:"max_page"`
	ActualPage int         `json:"actual_page"`
	PerPage    int         `json:"per_page"`
	Data       []*SafeUser `json:"data"`
	Error      string      `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, MessageEnvelope{Error: msg})
}

// httpError maps domain sentinel errors to HTTP status codes.
// Infrastructure errors (DynamoDB, S3, etc.) are hidden behind a generic 500 message.
func httpError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, domain.ErrBadRequest):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

// formatDate formats a time.Time as "yyyy-mm-dd". Returns "" for zero time.
func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}
