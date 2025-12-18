package transport

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Meldy183/code-storage-service/internal/domain"
	"github.com/Meldy183/code-storage-service/internal/service"
	"github.com/Meldy183/shared/pkg/logger"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

const maxUploadSize = 100 << 20 // 100MB

// Handler handles HTTP requests for the storage service
type Handler struct {
	service *service.Service
}

// NewHandler creates a new Handler instance
func NewHandler(svc *service.Service) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes registers all routes for the handler
func (h *Handler) RegisterRoutes(router *mux.Router, log logger.Logger) {
	router.Use(h.LoggingMiddleware(log))

	// Health check
	router.HandleFunc("/health", h.HealthCheck).Methods(http.MethodGet)

	// Storage endpoints
	router.HandleFunc("/storage/init", h.InitRepository).Methods(http.MethodPost)
	router.HandleFunc("/storage/push", h.Push).Methods(http.MethodPost)
	router.HandleFunc("/storage/checkout", h.Checkout).Methods(http.MethodGet)
	router.HandleFunc("/storage/merge", h.Merge).Methods(http.MethodPost)
	router.HandleFunc("/storage/commitName/{commit_id}", h.GetCommitName).Methods(http.MethodGet)
	router.HandleFunc("/storage/commitID", h.GetCommitIDByName).Methods(http.MethodGet)
}

// LoggingMiddleware adds logging and request ID to each request
func (h *Handler) LoggingMiddleware(log logger.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}
			w.Header().Set("X-Request-ID", requestID)
			ctx := logger.WithRequestID(r.Context(), requestID)
			ctx = logger.WithLogger(ctx, log)
			log.Info(ctx, "request received",
				zap.String("method", r.Method),
				zap.String("uri", r.RequestURI))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// InitRepository handles POST /storage/init
func (h *Handler) InitRepository(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	// Parse multipart form
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		log.Error(ctx, "failed to parse multipart form", zap.Error(err))
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "failed to parse form data")
		return
	}

	// Get team_id
	teamIDStr := r.FormValue("team_id")
	teamID, err := uuid.Parse(teamIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid team_id format")
		return
	}

	// Get commit_name (optional but recommended)
	commitName := r.FormValue("commit_name")

	// Get code file
	file, _, err := r.FormFile("code")
	if err != nil {
		log.Error(ctx, "failed to get code file", zap.Error(err))
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "code file is required")
		return
	}
	defer file.Close()

	code, err := io.ReadAll(file)
	if err != nil {
		log.Error(ctx, "failed to read code file", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read code file")
		return
	}

	// Initialize repository
	commit, err := h.service.InitRepository(ctx, teamID, commitName, code)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, domain.CommitResponse{Commit: commit.ToDTO()})
}

// Push handles POST /storage/push
func (h *Handler) Push(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	// Parse multipart form
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		log.Error(ctx, "failed to parse multipart form", zap.Error(err))
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "failed to parse form data")
		return
	}

	// Get team_id
	teamIDStr := r.FormValue("team_id")
	teamID, err := uuid.Parse(teamIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid team_id format")
		return
	}

	// Get root_commit
	rootCommitStr := r.FormValue("root_commit")
	rootCommit, err := uuid.Parse(rootCommitStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid root_commit format")
		return
	}

	// Get commit_id (parent commit)
	parentCommitIDStr := r.FormValue("commit_id")
	parentCommitID, err := uuid.Parse(parentCommitIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid commit_id format")
		return
	}

	// Get commit_name (optional but recommended)
	commitName := r.FormValue("commit_name")

	// Get code file
	file, _, err := r.FormFile("code")
	if err != nil {
		log.Error(ctx, "failed to get code file", zap.Error(err))
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "code file is required")
		return
	}
	defer file.Close()

	code, err := io.ReadAll(file)
	if err != nil {
		log.Error(ctx, "failed to read code file", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read code file")
		return
	}

	// Create commit
	commit, err := h.service.Push(ctx, teamID, rootCommit, parentCommitID, commitName, code)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, domain.CommitResponse{Commit: commit.ToDTO()})
}

// Checkout handles GET /storage/checkout
func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get query parameters
	teamIDStr := r.URL.Query().Get("team_id")
	teamID, err := uuid.Parse(teamIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid team_id format")
		return
	}

	rootCommitStr := r.URL.Query().Get("root_commit")
	rootCommit, err := uuid.Parse(rootCommitStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid root_commit format")
		return
	}

	commitIDStr := r.URL.Query().Get("commit_id")
	commitID, err := uuid.Parse(commitIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid commit_id format")
		return
	}

	// Get code
	code, err := h.service.Checkout(ctx, teamID, rootCommit, commitID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	// Return ZIP file
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=code.zip")
	w.WriteHeader(http.StatusOK)
	w.Write(code)
}

// Merge handles POST /storage/merge
func (h *Handler) Merge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req domain.MergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, "failed to decode request", zap.Error(err))
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	// Merge commits
	commit, err := h.service.Merge(ctx, req.TeamID, req.RootCommit, req.CommitID1, req.CommitID2)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, domain.CommitResponse{Commit: commit.ToDTO()})
}

// GetCommitName handles GET /storage/commitName/{commit_id}
func (h *Handler) GetCommitName(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	commitIDStr := vars["commit_id"]
	commitID, err := uuid.Parse(commitIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid commit_id format")
		return
	}

	name, err := h.service.GetCommitName(ctx, commitID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, domain.CommitNameResponse{
		CommitID: commitID,
		Name:     name,
	})
}

// GetCommitIDByName handles GET /storage/commitID
func (h *Handler) GetCommitIDByName(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	teamIDStr := r.URL.Query().Get("team_id")
	teamID, err := uuid.Parse(teamIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid team_id format")
		return
	}

	rootCommitStr := r.URL.Query().Get("root_commit")
	rootCommit, err := uuid.Parse(rootCommitStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid root_commit format")
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "name is required")
		return
	}

	commitID, err := h.service.GetCommitIDByName(ctx, teamID, rootCommit, name)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, domain.CommitIDResponse{CommitID: commitID})
}

// respondJSON sends a JSON response
func (h *Handler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// respondError sends an error response
func (h *Handler) respondError(w http.ResponseWriter, status int, code, message string) {
	h.respondJSON(w, status, domain.NewErrorResponse(code, message))
}

// handleServiceError maps service errors to HTTP responses
func (h *Handler) handleServiceError(w http.ResponseWriter, err error) {
	code := domain.MapErrorToCode(err)

	switch {
	case errors.Is(err, domain.ErrTeamNotFound):
		h.respondError(w, http.StatusNotFound, code, err.Error())
	case errors.Is(err, domain.ErrRootCommitNotFound):
		h.respondError(w, http.StatusNotFound, code, err.Error())
	case errors.Is(err, domain.ErrCommitNotFound):
		h.respondError(w, http.StatusNotFound, code, err.Error())
	case errors.Is(err, domain.ErrInvalidParent):
		h.respondError(w, http.StatusNotFound, code, err.Error())
	case errors.Is(err, domain.ErrCommitNotLeaf):
		h.respondError(w, http.StatusConflict, code, err.Error())
	case errors.Is(err, domain.ErrMergeConflict):
		h.respondError(w, http.StatusConflict, code, err.Error())
	case errors.Is(err, domain.ErrRepositoryAlreadyInit):
		h.respondError(w, http.StatusConflict, code, err.Error())
	case errors.Is(err, domain.ErrCommitNameExists):
		h.respondError(w, http.StatusConflict, code, err.Error())
	default:
		h.respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
