package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Meldy183/shared/pkg/logger"
	"github.com/Meldy183/user-gateway-service/internal/client"
	"github.com/Meldy183/user-gateway-service/internal/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service provides business logic for user gateway
type Service struct {
	prClient   *client.PRAllocationClient
	codeClient *client.CodeStorageClient
	// In-memory storage for PR metadata (in production would be in DB)
	prMetadata map[string]*PRMetadata
}

// PRMetadata stores additional PR info not in pr-allocation-service
type PRMetadata struct {
	RootCommit   uuid.UUID
	SourceCommit uuid.UUID
	TargetCommit uuid.UUID
	TeamID       uuid.UUID
}

// NewService creates a new Service instance
func NewService(prClient *client.PRAllocationClient, codeClient *client.CodeStorageClient) *Service {
	return &Service{
		prClient:   prClient,
		codeClient: codeClient,
		prMetadata: make(map[string]*PRMetadata),
	}
}

// GetUserProfile gets user profile by ID
func (s *Service) GetUserProfile(ctx context.Context, userID string) (*domain.UserProfile, error) {
	log := logger.FromContext(ctx)

	// Get user from pr-allocation-service
	user, err := s.prClient.GetUser(ctx, userID)
	if err != nil {
		log.Error(ctx, "failed to get user", zap.Error(err))
		return nil, domain.ErrUserNotFound
	}

	return &domain.UserProfile{
		UserID:   user.UserID,
		Username: user.Username,
		TeamID:   user.TeamID,
		TeamName: user.TeamName,
		IsActive: user.IsActive,
	}, nil
}

// ResolveTeamName resolves team name to team UUID
func (s *Service) ResolveTeamName(ctx context.Context, teamName string) (uuid.UUID, error) {
	return s.prClient.ResolveTeamID(ctx, teamName)
}

// GetUserTeamID gets user's team ID
func (s *Service) GetUserTeamID(ctx context.Context, userID string) (uuid.UUID, error) {
	user, err := s.prClient.GetUser(ctx, userID)
	if err != nil {
		return uuid.Nil, err
	}
	return user.TeamID, nil
}

// InitRepository initializes a new repository
func (s *Service) InitRepository(ctx context.Context, userID string, teamID uuid.UUID, code []byte) (*domain.Commit, error) {
	log := logger.FromContext(ctx)

	// Initialize repository in code-storage
	commit, err := s.codeClient.InitRepository(ctx, teamID, code)
	if err != nil {
		log.Error(ctx, "failed to init repository", zap.Error(err))
		if strings.Contains(err.Error(), "not found") {
			return nil, domain.ErrTeamNotFound
		}
		return nil, fmt.Errorf("failed to init repository: %w", err)
	}

	log.Info(ctx, "repository initialized",
		zap.String("user_id", userID),
		zap.String("root_commit", commit.RootCommit.String()),
	)

	return &domain.Commit{
		CommitID:        commit.CommitID,
		RootCommit:      commit.RootCommit,
		ParentCommitIDs: commit.ParentCommitIDs,
		CommitName:      commit.CommitName,
		CreatedAt:       commit.CreatedAt,
	}, nil
}

// Push creates a new commit
func (s *Service) Push(ctx context.Context, userID string, teamID, rootCommit, parentCommit uuid.UUID, code []byte) (*domain.Commit, error) {
	log := logger.FromContext(ctx)

	commit, err := s.codeClient.Push(ctx, teamID, rootCommit, parentCommit, code)
	if err != nil {
		log.Error(ctx, "failed to push commit", zap.Error(err))
		if strings.Contains(err.Error(), "not found") {
			return nil, domain.ErrCommitNotFound
		}
		return nil, fmt.Errorf("failed to push: %w", err)
	}

	log.Info(ctx, "commit pushed",
		zap.String("user_id", userID),
		zap.String("commit_id", commit.CommitID.String()),
	)

	return &domain.Commit{
		CommitID:        commit.CommitID,
		RootCommit:      commit.RootCommit,
		ParentCommitIDs: commit.ParentCommitIDs,
		CommitName:      commit.CommitName,
		CreatedAt:       commit.CreatedAt,
	}, nil
}

// Checkout retrieves code for a commit
func (s *Service) Checkout(ctx context.Context, userID string, teamID, rootCommit, commitID uuid.UUID) ([]byte, error) {
	log := logger.FromContext(ctx)

	code, err := s.codeClient.Checkout(ctx, teamID, rootCommit, commitID)
	if err != nil {
		log.Error(ctx, "failed to checkout", zap.Error(err))
		if strings.Contains(err.Error(), "not found") {
			return nil, domain.ErrCommitNotFound
		}
		return nil, fmt.Errorf("failed to checkout: %w", err)
	}

	log.Info(ctx, "checkout successful",
		zap.String("user_id", userID),
		zap.String("commit_id", commitID.String()),
	)

	return code, nil
}

// CreatePR creates a new pull request
func (s *Service) CreatePR(ctx context.Context, userID string, teamID uuid.UUID, req *domain.CreatePRRequest) (*domain.PullRequest, error) {
	log := logger.FromContext(ctx)

	// Generate PR ID
	prID := fmt.Sprintf("pr-%s", uuid.New().String()[:8])

	// Create PR in pr-allocation-service
	prResp, err := s.prClient.CreatePR(ctx, prID, req.Title, userID)
	if err != nil {
		log.Error(ctx, "failed to create PR", zap.Error(err))
		if strings.Contains(err.Error(), "already exists") {
			return nil, domain.ErrPRAlreadyExists
		}
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}

	// Store metadata for later use
	s.prMetadata[prID] = &PRMetadata{
		RootCommit:   req.RootCommit,
		SourceCommit: req.SourceCommit,
		TargetCommit: req.TargetCommit,
		TeamID:       teamID,
	}

	log.Info(ctx, "PR created",
		zap.String("pr_id", prID),
		zap.String("author_id", userID),
		zap.Strings("reviewers", prResp.AssignedReviewers),
	)

	return &domain.PullRequest{
		PRID:         prResp.PRID,
		Title:        prResp.PRName,
		AuthorID:     prResp.AuthorID,
		Status:       prResp.Status,
		ReviewerIDs:  prResp.AssignedReviewers,
		SourceCommit: req.SourceCommit,
		TargetCommit: req.TargetCommit,
		RootCommit:   req.RootCommit,
		CreatedAt:    prResp.CreatedAt,
	}, nil
}

// GetMyPRs gets PRs authored by user
func (s *Service) GetMyPRs(ctx context.Context, userID string, status string) ([]domain.PullRequest, error) {
	// This would need a new endpoint in pr-allocation-service
	// For now, return empty list
	return []domain.PullRequest{}, nil
}

// GetReviewPRs gets PRs where user is reviewer
func (s *Service) GetReviewPRs(ctx context.Context, userID string, status string) ([]domain.PullRequest, error) {
	log := logger.FromContext(ctx)

	prs, err := s.prClient.GetPRsByReviewer(ctx, userID)
	if err != nil {
		log.Error(ctx, "failed to get review PRs", zap.Error(err))
		return nil, fmt.Errorf("failed to get review PRs: %w", err)
	}

	result := make([]domain.PullRequest, 0, len(prs))
	for _, pr := range prs {
		if status != "" && pr.Status != status {
			continue
		}

		domainPR := domain.PullRequest{
			PRID:        pr.PRID,
			Title:       pr.PRName,
			AuthorID:    pr.AuthorID,
			Status:      pr.Status,
			ReviewerIDs: pr.AssignedReviewers,
			CreatedAt:   pr.CreatedAt,
			MergedAt:    pr.MergedAt,
		}

		// Add metadata if available
		if meta, ok := s.prMetadata[pr.PRID]; ok {
			domainPR.RootCommit = meta.RootCommit
			domainPR.SourceCommit = meta.SourceCommit
			domainPR.TargetCommit = meta.TargetCommit
		}

		result = append(result, domainPR)
	}

	return result, nil
}

// ApprovePR approves a PR and triggers merge
func (s *Service) ApprovePR(ctx context.Context, userID, prID string) (*domain.PullRequest, *domain.Commit, error) {
	log := logger.FromContext(ctx)

	// Get PR metadata
	meta, ok := s.prMetadata[prID]
	if !ok {
		return nil, nil, domain.ErrPRNotFound
	}

	// Verify user is a reviewer (would check pr-allocation-service)
	// For now, we trust the request

	// Merge commits in code-storage
	mergeCommit, err := s.codeClient.Merge(ctx, meta.TeamID, meta.RootCommit, meta.SourceCommit, meta.TargetCommit)
	if err != nil {
		log.Error(ctx, "failed to merge commits", zap.Error(err))
		return nil, nil, fmt.Errorf("failed to merge: %w", err)
	}

	// Mark PR as merged in pr-allocation-service
	prResp, err := s.prClient.MergePR(ctx, prID)
	if err != nil {
		log.Error(ctx, "failed to mark PR as merged", zap.Error(err))
		return nil, nil, fmt.Errorf("failed to update PR status: %w", err)
	}

	log.Info(ctx, "PR approved and merged",
		zap.String("pr_id", prID),
		zap.String("reviewer_id", userID),
		zap.String("merge_commit", mergeCommit.CommitID.String()),
	)

	pr := &domain.PullRequest{
		PRID:         prResp.PRID,
		Title:        prResp.PRName,
		AuthorID:     prResp.AuthorID,
		Status:       prResp.Status,
		ReviewerIDs:  prResp.AssignedReviewers,
		SourceCommit: meta.SourceCommit,
		TargetCommit: meta.TargetCommit,
		RootCommit:   meta.RootCommit,
		CreatedAt:    prResp.CreatedAt,
		MergedAt:     prResp.MergedAt,
	}

	commit := &domain.Commit{
		CommitID:        mergeCommit.CommitID,
		RootCommit:      mergeCommit.RootCommit,
		ParentCommitIDs: mergeCommit.ParentCommitIDs,
		CreatedAt:       mergeCommit.CreatedAt,
	}

	return pr, commit, nil
}

// RejectPR rejects a PR
func (s *Service) RejectPR(ctx context.Context, userID, prID string, reason string) (*domain.PullRequest, error) {
	log := logger.FromContext(ctx)

	// Get PR metadata
	meta, ok := s.prMetadata[prID]
	if !ok {
		return nil, domain.ErrPRNotFound
	}

	// pr-allocation-service doesn't have reject endpoint
	// We'd need to add it or handle this differently
	log.Info(ctx, "PR rejected",
		zap.String("pr_id", prID),
		zap.String("reviewer_id", userID),
		zap.String("reason", reason),
	)

	return &domain.PullRequest{
		PRID:         prID,
		Status:       "REJECTED",
		SourceCommit: meta.SourceCommit,
		TargetCommit: meta.TargetCommit,
		RootCommit:   meta.RootCommit,
		CreatedAt:    time.Now(),
	}, nil
}

// GetPRCode gets code for a PR
func (s *Service) GetPRCode(ctx context.Context, userID, prID string) ([]byte, error) {
	log := logger.FromContext(ctx)

	// Get PR metadata
	meta, ok := s.prMetadata[prID]
	if !ok {
		return nil, domain.ErrPRNotFound
	}

	code, err := s.codeClient.Checkout(ctx, meta.TeamID, meta.RootCommit, meta.SourceCommit)
	if err != nil {
		log.Error(ctx, "failed to get PR code", zap.Error(err))
		return nil, fmt.Errorf("failed to get PR code: %w", err)
	}

	return code, nil
}
