package http

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/meld/pr-allocation-service/internal/domain"
	"github.com/meld/pr-allocation-service/internal/service"
	"github.com/meld/pr-allocation-service/pkg/logger"
)

type Handler struct {
	service *service.Service
}

func NewHandler(svc *service.Service) *Handler {
	return &Handler{
		service: svc,
	}
}

func (h *Handler) RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		w.Header().Set("X-Request-ID", requestID)
		ctx := logger.WithRequestID(r.Context(), requestID)

		log := logger.FromContext(ctx)
		log.Debug(ctx, "incoming request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := logger.FromContext(ctx)
		log.Info(ctx, "request received",
			zap.String("method", r.Method),
			zap.String("uri", r.RequestURI))
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				ctx := r.Context()
				log := logger.FromContext(ctx)
				log.Error(ctx, "panic recovered", zap.Any("error", err))
				h.respondError(w, r, http.StatusInternalServerError, domain.ErrNotFound, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.Use(h.RequestIDMiddleware)
	router.Use(h.LoggingMiddleware)
	router.Use(h.RecoveryMiddleware)

	// Health
	router.HandleFunc("/health", h.HealthCheck).Methods("GET")

	// Teams - matching OpenAPI spec
	router.HandleFunc("/team/add", h.CreateTeam).Methods("POST")
	router.HandleFunc("/team/get", h.GetTeam).Methods("GET")

	// Users - matching OpenAPI spec
	router.HandleFunc("/users/setIsActive", h.SetUserActive).Methods("POST")
	router.HandleFunc("/users/getReview", h.GetPRsByReviewer).Methods("GET")

	// PRs - matching OpenAPI spec
	router.HandleFunc("/pullRequest/create", h.CreatePR).Methods("POST")
	router.HandleFunc("/pullRequest/merge", h.MergePR).Methods("POST")
	router.HandleFunc("/pullRequest/reassign", h.ReassignReviewer).Methods("POST")
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.respondJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /team/add
func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req domain.CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, "failed to decode request", zap.Error(err))
		h.respondError(w, r, http.StatusBadRequest, domain.ErrNotFound, "invalid request body")
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

// GET /team/get?team_name=...
func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		h.respondError(w, r, http.StatusBadRequest, domain.ErrNotFound, "team_name query parameter required")
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

// POST /users/setIsActive
func (h *Handler) SetUserActive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req domain.SetUserActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, "failed to decode request", zap.Error(err))
		h.respondError(w, r, http.StatusBadRequest, domain.ErrNotFound, "invalid request body")
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

// POST /pullRequest/create
func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req domain.CreatePRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, "failed to decode request", zap.Error(err))
		h.respondError(w, r, http.StatusBadRequest, domain.ErrNotFound, "invalid request body")
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

// POST /pullRequest/merge
func (h *Handler) MergePR(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req domain.MergePRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, "failed to decode request", zap.Error(err))
		h.respondError(w, r, http.StatusBadRequest, domain.ErrNotFound, "invalid request body")
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

// POST /pullRequest/reassign
func (h *Handler) ReassignReviewer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req domain.ReassignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, "failed to decode request", zap.Error(err))
		h.respondError(w, r, http.StatusBadRequest, domain.ErrNotFound, "invalid request body")
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

	response := map[string]interface{}{
		"pr":          pr,
		"replaced_by": newReviewerID,
	}
	h.respondJSON(w, r, http.StatusOK, response)
}

// GET /users/getReview?user_id=...
func (h *Handler) GetPRsByReviewer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		h.respondError(w, r, http.StatusBadRequest, domain.ErrNotFound, "user_id query parameter required")
		return
	}

	prs, err := h.service.GetPRsByReviewer(ctx, userID)
	if err != nil {
		log.Error(ctx, "failed to get PRs by reviewer", zap.Error(err))
		h.respondError(w, r, http.StatusInternalServerError, domain.ErrNotFound, err.Error())
		return
	}

	response := map[string]interface{}{
		"user_id":       userID,
		"pull_requests": prs,
	}
	h.respondJSON(w, r, http.StatusOK, response)
}

func (h *Handler) respondJSON(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
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
