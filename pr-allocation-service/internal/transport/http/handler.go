package http

import (
	"encoding/json"
	"github.com/meld/pr-allocation-service/pr-allocation-service/internal/domain"
	"github.com/meld/pr-allocation-service/pr-allocation-service/internal/service"
	"github.com/meld/pr-allocation-service/pr-allocation-service/pkg/logger"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type Handler struct {
	service *service.Service
}

func NewHandler(svc *service.Service) *Handler {
	return &Handler{
		service: svc,
	}
}

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

func (h *Handler) RegisterRoutes(router *mux.Router, log logger.Logger) {
	router.Use(h.LoggingMiddleware(log))

	// Health
	router.HandleFunc("/health", h.HealthCheck).Methods("GET")

	// Teams - matching OpenAPI spec
	router.HandleFunc("/team/add", h.CreateTeam).Methods("POST")
	router.HandleFunc("/team/get", h.GetTeam).Methods("GET")
	router.HandleFunc("/team/deactivateUsers", h.BulkDeactivateTeamUsers).Methods("POST")

	// Users - matching OpenAPI spec
	router.HandleFunc("/users/setIsActive", h.SetUserActive).Methods("POST")
	router.HandleFunc("/users/getReview", h.GetPRsByReviewer).Methods("GET")

	// PRs - matching OpenAPI spec
	router.HandleFunc("/pullRequest/create", h.CreatePR).Methods("POST")
	router.HandleFunc("/pullRequest/merge", h.MergePR).Methods("POST")
	router.HandleFunc("/pullRequest/reassign", h.ReassignReviewer).Methods("POST")

	// Statistics
	router.HandleFunc("/statistics", h.GetStatistics).Methods("GET")
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.respondJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

// CreateTeam POST /team/add.
func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	var req domain.CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, "failed to decode request", zap.Error(err))
		h.respondError(w, r, http.StatusBadRequest, domain.ErrInvalidRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.TeamName == "" {
		h.respondError(w, r, http.StatusBadRequest, domain.ErrInvalidRequest, "team_name is required")
		return
	}
	if len(req.Members) == 0 {
		h.respondError(w, r, http.StatusBadRequest, domain.ErrInvalidRequest, "at least one member is required")
		return
	}

	team, err := h.service.CreateTeam(ctx, &req)
	if err != nil {
		log.Error(ctx, "failed to create team", zap.Error(err))
		// Check if team exists error
		if contains(err.Error(), domain.ErrTeamExists) {
			h.respondError(w, r, http.StatusBadRequest, domain.ErrTeamExists, "team_name already exists")
			return
		}
		h.respondError(w, r, http.StatusInternalServerError, domain.ErrNotFound, err.Error())
		return
	}

	h.respondJSON(w, r, http.StatusCreated, map[string]*domain.Team{"team": team})
}

// GetTeam GET /team/get?team_name=...
func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		h.respondError(w, r, http.StatusBadRequest, domain.ErrInvalidRequest, "team_name query parameter required")
		return
	}

	team, err := h.service.GetTeam(ctx, teamName)
	if err != nil {
		log.Error(ctx, "failed to get team", zap.Error(err))
		h.respondError(w, r, http.StatusNotFound, domain.ErrNotFound, "team not found")
		return
	}

	h.respondJSON(w, r, http.StatusOK, team)
}

// SetUserActive POST /users/setIsActive.
func (h *Handler) SetUserActive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req domain.SetUserActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, "failed to decode request", zap.Error(err))
		h.respondError(w, r, http.StatusBadRequest, domain.ErrInvalidRequest, "invalid request body")
		return
	}

	if req.UserID == "" {
		h.respondError(w, r, http.StatusBadRequest, domain.ErrInvalidRequest, "user_id is required")
		return
	}

	user, err := h.service.SetUserActive(ctx, &req)
	if err != nil {
		log.Error(ctx, "failed to set user active", zap.Error(err))
		h.respondError(w, r, http.StatusNotFound, domain.ErrNotFound, "user not found")
		return
	}

	h.respondJSON(w, r, http.StatusOK, map[string]*domain.User{"user": user})
}

// CreatePR POST /pullRequest/create.
func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req domain.CreatePRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, "failed to decode request", zap.Error(err))
		h.respondError(w, r, http.StatusBadRequest, domain.ErrInvalidRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.PullRequestID == "" || req.PullRequestName == "" || req.AuthorID == "" {
		h.respondError(w, r, http.StatusBadRequest, domain.ErrInvalidRequest, "pull_request_id, pull_request_name, and author_id are required")
		return
	}

	pr, err := h.service.CreatePR(ctx, &req)
	if err != nil {
		log.Error(ctx, "failed to create PR", zap.Error(err))

		// Check specific error codes
		if contains(err.Error(), domain.ErrPRExists) {
			h.respondError(w, r, http.StatusConflict, domain.ErrPRExists, "PR id already exists")
			return
		}
		if contains(err.Error(), domain.ErrNotFound) {
			h.respondError(w, r, http.StatusNotFound, domain.ErrNotFound, "author or team not found")
			return
		}

		h.respondError(w, r, http.StatusInternalServerError, domain.ErrNotFound, err.Error())
		return
	}

	h.respondJSON(w, r, http.StatusCreated, map[string]*domain.PullRequest{"pr": pr})
}

// MergePR POST /pullRequest/merge.
func (h *Handler) MergePR(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req domain.MergePRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, "failed to decode request", zap.Error(err))
		h.respondError(w, r, http.StatusBadRequest, domain.ErrInvalidRequest, "invalid request body")
		return
	}

	if req.PullRequestID == "" {
		h.respondError(w, r, http.StatusBadRequest, domain.ErrInvalidRequest, "pull_request_id is required")
		return
	}

	pr, err := h.service.MergePR(ctx, &req)
	if err != nil {
		log.Error(ctx, "failed to merge PR", zap.Error(err))

		if contains(err.Error(), domain.ErrNotFound) {
			h.respondError(w, r, http.StatusNotFound, domain.ErrNotFound, "PR not found")
			return
		}

		h.respondError(w, r, http.StatusInternalServerError, domain.ErrNotFound, err.Error())
		return
	}

	h.respondJSON(w, r, http.StatusOK, map[string]*domain.PullRequest{"pr": pr})
}

// ReassignReviewer POST /pullRequest/reassign.
func (h *Handler) ReassignReviewer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req domain.ReassignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, "failed to decode request", zap.Error(err))
		h.respondError(w, r, http.StatusBadRequest, domain.ErrInvalidRequest, "invalid request body")
		return
	}

	if req.PullRequestID == "" || req.OldUserID == "" {
		h.respondError(w, r, http.StatusBadRequest, domain.ErrInvalidRequest, "pull_request_id and old_user_id are required")
		return
	}

	newReviewerID, pr, err := h.service.ReassignReviewer(ctx, &req)
	if err != nil {
		log.Error(ctx, "failed to reassign reviewer", zap.Error(err))

		// Check specific error codes
		if contains(err.Error(), domain.ErrPRMerged) {
			h.respondError(w, r, http.StatusConflict, domain.ErrPRMerged, "cannot reassign on merged PR")
			return
		}
		if contains(err.Error(), domain.ErrNotAssigned) {
			h.respondError(w, r, http.StatusConflict, domain.ErrNotAssigned, "reviewer is not assigned to this PR")
			return
		}
		if contains(err.Error(), domain.ErrNoCandidate) {
			h.respondError(w, r, http.StatusConflict, domain.ErrNoCandidate, "no active replacement candidate in team")
			return
		}
		if contains(err.Error(), domain.ErrNotFound) {
			h.respondError(w, r, http.StatusNotFound, domain.ErrNotFound, "PR or user not found")
			return
		}

		h.respondError(w, r, http.StatusInternalServerError, domain.ErrNotFound, err.Error())
		return
	}

	response := map[string]any{
		"pr":          pr,
		"replaced_by": newReviewerID,
	}
	h.respondJSON(w, r, http.StatusOK, response)
}

// GetPRsByReviewer GET /users/getReview?user_id=...
func (h *Handler) GetPRsByReviewer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		h.respondError(w, r, http.StatusBadRequest, domain.ErrInvalidRequest, "user_id query parameter required")
		return
	}

	prs, err := h.service.GetPRsByReviewer(ctx, userID)
	if err != nil {
		log.Error(ctx, "failed to get PRs by reviewer", zap.Error(err))
		h.respondError(w, r, http.StatusInternalServerError, domain.ErrNotFound, err.Error())
		return
	}

	response := map[string]any{
		"user_id":       userID,
		"pull_requests": prs,
	}
	h.respondJSON(w, r, http.StatusOK, response)
}

func (h *Handler) respondJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		ctx := r.Context()
		log := logger.FromContext(ctx)
		log.Error(ctx, "failed to encode response", zap.Error(err))
	}
}

func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	errorResponse := domain.ErrorResponse{
		Error: domain.ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
	h.respondJSON(w, r, status, errorResponse)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

// GetStatistics GET /statistics
func (h *Handler) GetStatistics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	stats, err := h.service.GetStatistics(ctx)
	if err != nil {
		log.Error(ctx, "failed to get statistics", zap.Error(err))
		h.respondError(w, r, http.StatusInternalServerError, domain.ErrNotFound, err.Error())
		return
	}
	h.respondJSON(w, r, http.StatusOK, stats)
}

// BulkDeactivateTeamUsers POST /team/deactivateUsers
func (h *Handler) BulkDeactivateTeamUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	var req domain.BulkDeactivateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, "failed to decode request", zap.Error(err))
		h.respondError(w, r, http.StatusBadRequest, domain.ErrInvalidRequest, "invalid request body")
		return
	}
	if req.TeamName == "" {
		h.respondError(w, r, http.StatusBadRequest, domain.ErrInvalidRequest, "team_name is required")
		return
	}
	response, err := h.service.BulkDeactivateTeamUsers(ctx, &req)
	if err != nil {
		log.Error(ctx, "failed to bulk deactivate team users", zap.Error(err))
		if contains(err.Error(), domain.ErrNotFound) {
			h.respondError(w, r, http.StatusNotFound, domain.ErrNotFound, "team not found")
			return
		}
		h.respondError(w, r, http.StatusInternalServerError, domain.ErrNotFound, err.Error())
		return
	}
	h.respondJSON(w, r, http.StatusOK, response)
}
