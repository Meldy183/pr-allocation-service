package transport

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Meldy183/shared/pkg/logger"
	"github.com/Meldy183/user-gateway-service/internal/domain"
	"github.com/Meldy183/user-gateway-service/internal/service"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

const maxUploadSize = 100 << 20 // 100MB

// Handler handles HTTP requests
type Handler struct {
	service *service.Service
}

// NewHandler creates a new Handler instance
func NewHandler(svc *service.Service) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes registers all routes
func (h *Handler) RegisterRoutes(router *mux.Router, log logger.Logger) {
	router.Use(h.LoggingMiddleware(log))

	// Health
	router.HandleFunc("/health", h.HealthCheck).Methods(http.MethodGet)

	// Profile
	router.HandleFunc("/api/me", h.GetProfile).Methods(http.MethodGet)

	// Teams
	router.HandleFunc("/api/team/create", h.CreateTeam).Methods(http.MethodPost)
	router.HandleFunc("/api/team/get", h.GetTeam).Methods(http.MethodGet)

	// Repository
	router.HandleFunc("/api/repo/init", h.InitRepository).Methods(http.MethodPost)
	router.HandleFunc("/api/repo/push", h.Push).Methods(http.MethodPost)
	router.HandleFunc("/api/repo/checkout", h.Checkout).Methods(http.MethodGet)

	// Pull Requests - now using query params instead of path params
	router.HandleFunc("/api/pr/create", h.CreatePR).Methods(http.MethodPost)
	router.HandleFunc("/api/pr/my", h.GetMyPRs).Methods(http.MethodGet)
	router.HandleFunc("/api/pr/reviews", h.GetReviewPRs).Methods(http.MethodGet)
	router.HandleFunc("/api/pr/approve", h.ApprovePR).Methods(http.MethodPost)
	router.HandleFunc("/api/pr/reject", h.RejectPR).Methods(http.MethodPost)
	router.HandleFunc("/api/pr/code", h.GetPRCode).Methods(http.MethodGet)
}

// LoggingMiddleware adds logging and request ID
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

// getUsername extracts username from header
func (h *Handler) getUsername(r *http.Request) string {
	return r.Header.Get("X-Username")
}

// HealthCheck handles health check
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// CreateTeam handles POST /api/team/create
func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	username := h.getUsername(r)

	if username == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-Username header is required")
		return
	}

	var req domain.CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, "failed to decode request", zap.Error(err))
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid request body")
		return
	}

	if req.TeamName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "team_name is required")
		return
	}
	if len(req.Members) == 0 {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "at least one member is required")
		return
	}

	team, err := h.service.CreateTeam(ctx, &req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, map[string]interface{}{"team": team})
}

// GetTeam handles GET /api/team/get
func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := h.getUsername(r)

	if username == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-Username header is required")
		return
	}

	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "team_name query parameter is required")
		return
	}

	team, err := h.service.GetTeam(ctx, teamName)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{"team": team})
}

// GetProfile handles GET /api/me
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := h.getUsername(r)

	if username == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-Username header is required")
		return
	}

	profile, err := h.service.GetUserProfile(ctx, username)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{"user": profile})
}

// InitRepository handles POST /api/repo/init
func (h *Handler) InitRepository(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	username := h.getUsername(r)

	if username == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-Username header is required")
		return
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		log.Error(ctx, "failed to parse multipart form", zap.Error(err))
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "failed to parse form data")
		return
	}

	// Get team_name from form
	teamName := r.FormValue("team_name")
	if teamName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "team_name is required")
		return
	}

	// Get repo_name from form
	repoName := r.FormValue("repo_name")
	if repoName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "repo_name is required")
		return
	}

	// Get commit_name from form
	commitName := r.FormValue("commit_name")
	if commitName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "commit_name is required")
		return
	}

	// Get code file
	file, _, err := r.FormFile("code")
	if err != nil {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "code file is required")
		return
	}
	defer file.Close()

	code, err := io.ReadAll(file)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, domain.ErrCodeInternalError, "failed to read code file")
		return
	}

	commit, err := h.service.InitRepository(ctx, username, teamName, repoName, commitName, code)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, map[string]interface{}{"commit": commit})
}

// Push handles POST /api/repo/push
func (h *Handler) Push(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	username := h.getUsername(r)

	if username == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-Username header is required")
		return
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		log.Error(ctx, "failed to parse multipart form", zap.Error(err))
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "failed to parse form data")
		return
	}

	teamName := r.FormValue("team_name")
	if teamName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "team_name is required")
		return
	}

	repoName := r.FormValue("repo_name")
	if repoName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "repo_name is required")
		return
	}

	parentCommitName := r.FormValue("parent_commit_name")
	if parentCommitName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "parent_commit_name is required")
		return
	}

	commitName := r.FormValue("commit_name")
	if commitName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "commit_name is required")
		return
	}

	file, _, err := r.FormFile("code")
	if err != nil {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "code file is required")
		return
	}
	defer file.Close()

	code, err := io.ReadAll(file)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, domain.ErrCodeInternalError, "failed to read code file")
		return
	}

	commit, err := h.service.Push(ctx, username, teamName, repoName, parentCommitName, commitName, code)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, map[string]interface{}{"commit": commit})
}

// Checkout handles GET /api/repo/checkout
func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := h.getUsername(r)

	if username == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-Username header is required")
		return
	}

	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "team_name is required")
		return
	}

	repoName := r.URL.Query().Get("repo_name")
	if repoName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "repo_name is required")
		return
	}

	commitName := r.URL.Query().Get("commit_name")
	if commitName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "commit_name is required")
		return
	}

	code, err := h.service.Checkout(ctx, username, teamName, repoName, commitName)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=code.zip")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(code)
}

// CreatePR handles POST /api/pr/create
func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	username := h.getUsername(r)

	if username == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-Username header is required")
		return
	}

	var req domain.CreatePRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, "failed to decode request", zap.Error(err))
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid request body")
		return
	}

	if req.Title == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "title is required")
		return
	}
	if req.PRName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "pr_name is required")
		return
	}
	if req.TeamName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "team_name is required")
		return
	}
	if req.RepoName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "repo_name is required")
		return
	}
	if req.SourceCommitName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "source_commit_name is required")
		return
	}
	if req.TargetCommitName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "target_commit_name is required")
		return
	}

	pr, err := h.service.CreatePR(ctx, username, &req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, map[string]interface{}{"pull_request": pr})
}

// GetMyPRs handles GET /api/pr/my
func (h *Handler) GetMyPRs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := h.getUsername(r)

	if username == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-Username header is required")
		return
	}

	status := r.URL.Query().Get("status")

	prs, err := h.service.GetMyPRs(ctx, username, status)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{"pull_requests": prs})
}

// GetReviewPRs handles GET /api/pr/reviews
func (h *Handler) GetReviewPRs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := h.getUsername(r)

	if username == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-Username header is required")
		return
	}

	status := r.URL.Query().Get("status")

	prs, err := h.service.GetReviewPRs(ctx, username, status)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{"pull_requests": prs})
}

// ApprovePR handles POST /api/pr/approve
func (h *Handler) ApprovePR(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := h.getUsername(r)

	if username == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-Username header is required")
		return
	}

	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "team_name is required")
		return
	}

	prName := r.URL.Query().Get("pr_name")
	if prName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "pr_name is required")
		return
	}

	pr, mergeCommit, err := h.service.ApprovePR(ctx, username, teamName, prName)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"pull_request": pr,
		"merge_commit": mergeCommit,
	})
}

// RejectPR handles POST /api/pr/reject
func (h *Handler) RejectPR(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	username := h.getUsername(r)

	if username == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-Username header is required")
		return
	}

	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "team_name is required")
		return
	}

	prName := r.URL.Query().Get("pr_name")
	if prName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "pr_name is required")
		return
	}

	var req domain.RejectPRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug(ctx, "no reject reason provided")
	}

	pr, err := h.service.RejectPR(ctx, username, teamName, prName, req.Reason)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{"pull_request": pr})
}

// GetPRCode handles GET /api/pr/code
func (h *Handler) GetPRCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := h.getUsername(r)

	if username == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-Username header is required")
		return
	}

	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "team_name is required")
		return
	}

	prName := r.URL.Query().Get("pr_name")
	if prName == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "pr_name is required")
		return
	}

	code, err := h.service.GetPRCode(ctx, username, teamName, prName)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=code.zip")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(code)
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
	case errors.Is(err, domain.ErrAccessDenied):
		h.respondError(w, http.StatusForbidden, code, err.Error())
	case errors.Is(err, domain.ErrUserNotFound):
		h.respondError(w, http.StatusNotFound, code, err.Error())
	case errors.Is(err, domain.ErrUserInactive):
		h.respondError(w, http.StatusForbidden, code, err.Error())
	case errors.Is(err, domain.ErrTeamNotFound):
		h.respondError(w, http.StatusNotFound, code, err.Error())
	case errors.Is(err, domain.ErrTeamExists):
		h.respondError(w, http.StatusBadRequest, code, err.Error())
	case errors.Is(err, domain.ErrCommitNotFound):
		h.respondError(w, http.StatusNotFound, code, err.Error())
	case errors.Is(err, domain.ErrPRNotFound):
		h.respondError(w, http.StatusNotFound, code, err.Error())
	case errors.Is(err, domain.ErrPRAlreadyExists):
		h.respondError(w, http.StatusConflict, code, err.Error())
	case errors.Is(err, domain.ErrPRAlreadyMerged):
		h.respondError(w, http.StatusConflict, code, err.Error())
	case errors.Is(err, domain.ErrNotReviewer):
		h.respondError(w, http.StatusForbidden, code, err.Error())
	default:
		h.respondError(w, http.StatusInternalServerError, domain.ErrCodeInternalError, "internal server error")
	}
}
