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

	// Pull Requests
	router.HandleFunc("/api/pr/create", h.CreatePR).Methods(http.MethodPost)
	router.HandleFunc("/api/pr/my", h.GetMyPRs).Methods(http.MethodGet)
	router.HandleFunc("/api/pr/reviews", h.GetReviewPRs).Methods(http.MethodGet)
	router.HandleFunc("/api/pr/{pr_id}/approve", h.ApprovePR).Methods(http.MethodPost)
	router.HandleFunc("/api/pr/{pr_id}/reject", h.RejectPR).Methods(http.MethodPost)
	router.HandleFunc("/api/pr/{pr_id}/code", h.GetPRCode).Methods(http.MethodGet)
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

// getUserID extracts user ID from header
func (h *Handler) getUserID(r *http.Request) string {
	return r.Header.Get("X-User-ID")
}

// HealthCheck handles health check
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// CreateTeam handles POST /api/team/create
func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	userID := h.getUserID(r)

	if userID == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-User-ID header is required")
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
	userID := h.getUserID(r)

	if userID == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-User-ID header is required")
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
	userID := h.getUserID(r)

	if userID == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-User-ID header is required")
		return
	}

	profile, err := h.service.GetUserProfile(ctx, userID)
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
	userID := h.getUserID(r)

	if userID == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-User-ID header is required")
		return
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		log.Error(ctx, "failed to parse multipart form", zap.Error(err))
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "failed to parse form data")
		return
	}

	// Get team_id from form
	teamIDStr := r.FormValue("team_id")
	teamID, err := uuid.Parse(teamIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid team_id format")
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

	commit, err := h.service.InitRepository(ctx, userID, teamID, code)
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
	userID := h.getUserID(r)

	if userID == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-User-ID header is required")
		return
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		log.Error(ctx, "failed to parse multipart form", zap.Error(err))
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "failed to parse form data")
		return
	}

	teamIDStr := r.FormValue("team_id")
	teamID, err := uuid.Parse(teamIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid team_id format")
		return
	}

	rootCommitStr := r.FormValue("root_commit")
	rootCommit, err := uuid.Parse(rootCommitStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid root_commit format")
		return
	}

	parentCommitStr := r.FormValue("parent_commit")
	parentCommit, err := uuid.Parse(parentCommitStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid parent_commit format")
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

	commit, err := h.service.Push(ctx, userID, teamID, rootCommit, parentCommit, code)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, map[string]interface{}{"commit": commit})
}

// Checkout handles GET /api/repo/checkout
func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := h.getUserID(r)

	if userID == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-User-ID header is required")
		return
	}

	teamIDStr := r.URL.Query().Get("team_id")
	teamID, err := uuid.Parse(teamIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid team_id format")
		return
	}

	rootCommitStr := r.URL.Query().Get("root_commit")
	rootCommit, err := uuid.Parse(rootCommitStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid root_commit format")
		return
	}

	commitIDStr := r.URL.Query().Get("commit_id")
	commitID, err := uuid.Parse(commitIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid commit_id format")
		return
	}

	code, err := h.service.Checkout(ctx, userID, teamID, rootCommit, commitID)
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
	userID := h.getUserID(r)

	if userID == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-User-ID header is required")
		return
	}

	var req domain.CreatePRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, "failed to decode request", zap.Error(err))
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid request body")
		return
	}

	// Get team_id from query or use a default
	teamIDStr := r.URL.Query().Get("team_id")
	var teamID uuid.UUID
	if teamIDStr != "" {
		var err error
		teamID, err = uuid.Parse(teamIDStr)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid team_id format")
			return
		}
	}

	pr, err := h.service.CreatePR(ctx, userID, teamID, &req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, map[string]interface{}{"pull_request": pr})
}

// GetMyPRs handles GET /api/pr/my
func (h *Handler) GetMyPRs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := h.getUserID(r)

	if userID == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-User-ID header is required")
		return
	}

	status := r.URL.Query().Get("status")

	prs, err := h.service.GetMyPRs(ctx, userID, status)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{"pull_requests": prs})
}

// GetReviewPRs handles GET /api/pr/reviews
func (h *Handler) GetReviewPRs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := h.getUserID(r)

	if userID == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-User-ID header is required")
		return
	}

	status := r.URL.Query().Get("status")

	prs, err := h.service.GetReviewPRs(ctx, userID, status)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{"pull_requests": prs})
}

// ApprovePR handles POST /api/pr/{pr_id}/approve
func (h *Handler) ApprovePR(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := h.getUserID(r)

	if userID == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-User-ID header is required")
		return
	}

	vars := mux.Vars(r)
	prID := vars["pr_id"]

	pr, mergeCommit, err := h.service.ApprovePR(ctx, userID, prID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"pull_request": pr,
		"merge_commit": mergeCommit,
	})
}

// RejectPR handles POST /api/pr/{pr_id}/reject
func (h *Handler) RejectPR(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	userID := h.getUserID(r)

	if userID == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-User-ID header is required")
		return
	}

	vars := mux.Vars(r)
	prID := vars["pr_id"]

	var req domain.RejectPRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug(ctx, "no reject reason provided")
	}

	pr, err := h.service.RejectPR(ctx, userID, prID, req.Reason)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{"pull_request": pr})
}

// GetPRCode handles GET /api/pr/{pr_id}/code
func (h *Handler) GetPRCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := h.getUserID(r)

	if userID == "" {
		h.respondError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "X-User-ID header is required")
		return
	}

	vars := mux.Vars(r)
	prID := vars["pr_id"]

	code, err := h.service.GetPRCode(ctx, userID, prID)
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
