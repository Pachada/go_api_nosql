package handler

import (
	"encoding/json"
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
	RoleID         string    `json:"role_id"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Birthday       time.Time `json:"birthday,omitempty"`
	Verified       bool      `json:"verified"`
	EmailConfirmed bool      `json:"email_confirmed"`
	PhoneConfirmed bool      `json:"phone_confirmed"`
	Enable         bool      `json:"enable"`
	CreatedAt      time.Time `json:"created"`
	UpdatedAt      time.Time `json:"updated"`
}

// SafeSession is the public-facing session DTO that omits RefreshToken and RefreshExpiresAt.
type SafeSession struct {
	SessionID string    `json:"id"`
	UserID    string    `json:"user_id"`
	DeviceID  string    `json:"device_id,omitempty"`
	Enable    bool      `json:"enable"`
	CreatedAt time.Time `json:"created"`
	UpdatedAt time.Time `json:"updated"`
	User      *SafeUser `json:"user,omitempty"`
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
		RoleID:         u.RoleID,
		FirstName:      u.FirstName,
		LastName:       u.LastName,
		Birthday:       u.Birthday,
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
	ss := &SafeSession{
		SessionID: s.SessionID,
		UserID:    s.UserID,
		DeviceID:  s.DeviceID,
		Enable:    s.Enable,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
	if s.User != nil {
		ss.User = toSafeUser(s.User)
	}
	return ss
}

// MessageEnvelope is the generic response wrapper.
type MessageEnvelope struct {
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
	ErrorCode int    `json:"error_code,omitempty"`
}

// AuthEnvelope wraps login/register responses.
type AuthEnvelope struct {
	Bearer       string       `json:"Bearer,omitempty"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	Session      *SafeSession `json:"session,omitempty"`
	Message      string       `json:"message,omitempty"`
	Error        string       `json:"error,omitempty"`
}

// SessionEnvelope wraps current-session responses.
type SessionEnvelope struct {
	Session *SafeSession `json:"session,omitempty"`
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
