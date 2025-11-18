package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/meld/pr-allocation-service/internal/domain"
	"github.com/meld/pr-allocation-service/internal/storage"
	"github.com/meld/pr-allocation-service/pkg/logger"
	"go.uber.org/zap"
)

type Service struct {
	storage storage.Storage
}

func NewService(storage storage.Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// CreateTeam creates a team with members (POST /team/add).
func (s *Service) CreateTeam(ctx context.Context, req *domain.CreateTeamRequest) (*domain.Team, error) {
	log := logger.FromContext(ctx)
	log.Info(ctx, "creating team", zap.String("team_name", req.TeamName))
	exists, err := s.storage.TeamExists(ctx, req.TeamName)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("%s: team already exists", domain.ErrTeamExists)
	}
	team := &domain.Team{
		TeamName: req.TeamName,
		Members:  req.Members,
	}
	if err := s.storage.CreateTeam(ctx, team); err != nil {
		log.Error(ctx, "failed to create team", zap.Error(err))
		return nil, err
	}
	return team, nil
}

// GetTeam returns team with members (GET /team/get).
func (s *Service) GetTeam(ctx context.Context, teamName string) (*domain.Team, error) {
	return s.storage.GetTeam(ctx, teamName)
}

// SetUserActive updates user active status (POST /users/setIsActive).
func (s *Service) SetUserActive(ctx context.Context, req *domain.SetUserActiveRequest) (*domain.User, error) {
	log := logger.FromContext(ctx)
	log.Info(ctx, "setting user active status", zap.String("user_id", req.UserID), zap.Bool("is_active", req.IsActive))
	user, err := s.storage.GetUser(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	user.IsActive = req.IsActive
	if err := s.storage.UpdateUser(ctx, user); err != nil {
		log.Error(ctx, "failed to update user", zap.Error(err))
		return nil, err
	}
	return user, nil
}

// CreatePR creates PR and auto-assigns up to 2 reviewers (POST /pullRequest/create).
func (s *Service) CreatePR(ctx context.Context, req *domain.CreatePRRequest) (*domain.PullRequest, error) {
	log := logger.FromContext(ctx)
	log.Info(ctx, "creating PR", zap.String("pr_id", req.PullRequestID), zap.String("author_id", req.AuthorID))
	exists, err := s.storage.PRExists(ctx, req.PullRequestID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("%s: PR already exists", domain.ErrPRExists)
	}
	author, err := s.storage.GetUser(ctx, req.AuthorID)
	if err != nil {
		return nil, fmt.Errorf("%s: author not found", domain.ErrNotFound)
	}
	if author.TeamName == "" {
		return nil, fmt.Errorf("%s: author has no team", domain.ErrNotFound)
	}
	teamMembers, err := s.storage.GetUsersByTeam(ctx, author.TeamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}
	reviewers := s.selectReviewers(teamMembers, author.UserID, 2)
	pr := &domain.PullRequest{
		PullRequestID:     req.PullRequestID,
		PullRequestName:   req.PullRequestName,
		AuthorID:          req.AuthorID,
		AssignedReviewers: reviewers,
	}
	if err := s.storage.CreatePR(ctx, pr); err != nil {
		log.Error(ctx, "failed to create PR", zap.Error(err))
		return nil, err
	}
	log.Info(
		ctx,
		"PR created with reviewers",
		zap.String("pr_id", pr.PullRequestID),
		zap.Strings("reviewers", pr.AssignedReviewers),
	)
	return pr, nil
}

// MergePR marks PR as MERGED (POST /pullRequest/merge) - idempotent.
func (s *Service) MergePR(ctx context.Context, req *domain.MergePRRequest) (*domain.PullRequest, error) {
	log := logger.FromContext(ctx)
	log.Info(ctx, "merging PR", zap.String("pr_id", req.PullRequestID))
	pr, err := s.storage.GetPR(ctx, req.PullRequestID)
	if err != nil {
		return nil, fmt.Errorf("%s: PR not found", domain.ErrNotFound)
	}
	if pr.Status == domain.StatusMerged {
		log.Info(ctx, "PR already merged", zap.String("pr_id", req.PullRequestID))
		return pr, nil
	}
	pr.Status = domain.StatusMerged
	if pr.MergedAt == nil {
		now := time.Now()
		pr.MergedAt = &now
	}
	if err := s.storage.UpdatePR(ctx, pr); err != nil {
		log.Error(ctx, "failed to merge PR", zap.Error(err))
		return nil, err
	}
	return pr, nil
}

// ReassignReviewer replaces one reviewer with random active from their team.
func (s *Service) ReassignReviewer(
	ctx context.Context,
	req *domain.ReassignRequest,
) (string, *domain.PullRequest, error) {
	log := logger.FromContext(ctx)
	log.Info(
		ctx,
		"reassigning reviewer",
		zap.String("pr_id", req.PullRequestID),
		zap.String("old_user_id", req.OldUserID),
	)
	pr, err := s.storage.GetPR(ctx, req.PullRequestID)
	if err != nil {
		return "", nil, fmt.Errorf("%s: PR not found", domain.ErrNotFound)
	}
	if pr.Status == domain.StatusMerged {
		return "", nil, fmt.Errorf("%s: cannot reassign reviewers for merged PR", domain.ErrPRMerged)
	}
	found := false
	oldIndex := -1
	for i, rid := range pr.AssignedReviewers {
		if rid == req.OldUserID {
			found = true
			oldIndex = i
			break
		}
	}
	if !found {
		return "", nil, fmt.Errorf("%s: reviewer not assigned to this PR", domain.ErrNotAssigned)
	}
	oldReviewer, err := s.storage.GetUser(ctx, req.OldUserID)
	if err != nil {
		return "", nil, fmt.Errorf("%s: old reviewer not found", domain.ErrNotFound)
	}
	teamMembers, err := s.storage.GetUsersByTeam(ctx, oldReviewer.TeamName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get team members: %w", err)
	}
	excludeIDs := make(map[string]bool)
	excludeIDs[pr.AuthorID] = true
	for _, rid := range pr.AssignedReviewers {
		excludeIDs[rid] = true
	}
	var candidates []*domain.User
	for _, member := range teamMembers {
		if member.IsActive && !excludeIDs[member.UserID] {
			candidates = append(candidates, member)
		}
	}
	if len(candidates) == 0 {
		return "", nil, fmt.Errorf("%s: no active replacement candidate in team", domain.ErrNoCandidate)
	}
	newReviewer := candidates[rand.Intn(len(candidates))]
	pr.AssignedReviewers[oldIndex] = newReviewer.UserID
	if err := s.storage.UpdatePR(ctx, pr); err != nil {
		log.Error(ctx, "failed to reassign reviewer", zap.Error(err))
		return "", nil, err
	}
	log.Info(ctx, "reviewer reassigned", zap.String("pr_id", req.PullRequestID),
		zap.String("old", req.OldUserID), zap.String("new", newReviewer.UserID))
	return newReviewer.UserID, pr, nil
}

// GetPRsByReviewer returns PRs where user is assigned reviewer.
func (s *Service) GetPRsByReviewer(ctx context.Context, userID string) ([]*domain.PullRequestShort, error) {
	prs, err := s.storage.GetPRsByReviewer(ctx, userID)
	if err != nil {
		return nil, err
	}
	shorts := make([]*domain.PullRequestShort, len(prs))
	for i, pr := range prs {
		shorts[i] = &domain.PullRequestShort{
			PullRequestID:   pr.PullRequestID,
			PullRequestName: pr.PullRequestName,
			AuthorID:        pr.AuthorID,
			Status:          pr.Status,
		}
	}
	return shorts, nil
}
func (s *Service) selectReviewers(teamMembers []*domain.User, authorID string, maxCount int) []string {
	candidates := make([]*domain.User, 0)
	for _, member := range teamMembers {
		if member.IsActive && member.UserID != authorID {
			candidates = append(candidates, member)
		}
	}
	if len(candidates) == 0 {
		return []string{}
	}
	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
	count := min(len(candidates), maxCount)
	reviewers := make([]string, count)
	for i := range count {
		reviewers[i] = candidates[i].UserID
	}
	return reviewers
}
