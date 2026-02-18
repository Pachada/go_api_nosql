package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	fileapp "github.com/go-api-nosql/internal/application/file"
	"github.com/go-api-nosql/internal/transport/http/middleware"
)

// FileHandler handles S3 file endpoints.
type FileHandler struct {
	svc fileapp.Service
}

func NewFileHandler(svc fileapp.Service) *FileHandler { return &FileHandler{svc: svc} }

func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	f, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer f.Close()

	uploaded, err := h.svc.Upload(r.Context(), fileapp.UploadInput{
		Reader:      f,
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Size:        header.Size,
		IsPrivate:   r.URL.Query().Get("private") == "True",
		IsThumbnail: r.URL.Query().Get("thumbnail") == "True",
		UploaderID:  claims.UserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, uploaded)
}

func (h *FileHandler) UploadBase64(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		FileName string `json:"file_name"`
		Base64   string `json:"base64"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	uploaded, err := h.svc.UploadBase64(r.Context(), body.FileName, body.Base64, claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, uploaded)
}

func (h *FileHandler) Download(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	rc, _, err := h.svc.Download(r.Context(), chi.URLParam(r, "id"), claims.UserID, false)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = io.Copy(w, rc)
}

func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.svc.Delete(r.Context(), chi.URLParam(r, "id"), claims.UserID, false); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, MessageEnvelope{Message: "file deleted"})
}

func (h *FileHandler) ListBase64(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, MessageEnvelope{Message: "not implemented"})
}

func (h *FileHandler) GetBase64(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	f, b64, err := h.svc.GetBase64(r.Context(), chi.URLParam(r, "id"), claims.UserID, false)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"file": f, "base64": b64})
}

func (h *FileHandler) MethodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed when id is provided")
}

