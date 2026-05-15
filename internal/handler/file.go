package handler

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"escrow/internal/middleware"
)

type FileHandler struct {
	db       *pgxpool.Pool
	uploadDir string
}

func NewFileHandler(db *pgxpool.Pool, uploadDir string) *FileHandler {
	return &FileHandler{db: db, uploadDir: uploadDir}
}

func (h *FileHandler) Download(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	fileID := chi.URLParam(r, "id")
	if fileID == "" {
		http.Error(w, "File ID required", http.StatusBadRequest)
		return
	}

	var contractID, storedName, originalName string
	var clientID, freelancerID string
	err := h.db.QueryRow(r.Context(),
		`SELECT f.contract_id, f.stored_name, f.original_name,
		        c.client_id, c.freelancer_id
		 FROM files f
		 JOIN contracts c ON c.id = f.contract_id
		 WHERE f.id = $1`, fileID,
	).Scan(&contractID, &storedName, &originalName, &clientID, &freelancerID)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	if user.ID != clientID && user.ID != freelancerID && user.Role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	path := filepath.Join(h.uploadDir, storedName)
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "File not found on disk", http.StatusNotFound)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Disposition", "attachment; filename=\""+originalName+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")
	io.Copy(w, f)
}
