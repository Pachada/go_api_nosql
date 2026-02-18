package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-api-nosql/internal/domain"
)

// MessageEnvelope is the generic response wrapper.
type MessageEnvelope struct {
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
	ErrorCode int    `json:"error_code,omitempty"`
}

// AuthEnvelope wraps login/register responses.
type AuthEnvelope struct {
	Bearer  string          `json:"Bearer,omitempty"`
	Session *domain.Session `json:"session,omitempty"`
	Message string          `json:"message,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// SessionEnvelope wraps current-session responses.
type SessionEnvelope struct {
	Session *domain.Session `json:"session,omitempty"`
	Message string          `json:"message,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// PaginatedUsersEnvelope wraps paginated user list responses.
type PaginatedUsersEnvelope struct {
	MaxPage    int           `json:"max_page"`
	ActualPage int           `json:"actual_page"`
	PerPage    int           `json:"per_page"`
	Data       []domain.User `json:"data"`
	Error      string        `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, MessageEnvelope{Error: msg})
}
