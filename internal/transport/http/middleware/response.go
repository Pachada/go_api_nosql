package middleware

import (
	"encoding/json"
	"net/http"
)

// writeJSONError writes a JSON-encoded error response with the correct Content-Type.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
