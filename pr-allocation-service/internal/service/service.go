package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/Meldy183/pr-allocation-service/internal/domain"
	"github.com/Meldy183/pr-allocation-service/internal/storage"
	"github.com/Meldy183/shared/pkg/logger"
	"github.com/google/uuid"

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

// GetPR returns a PR by ID.
func (s *Service) GetPR(ctx context.Context, prID string) (*domain.PullRequest, error) {
	return s.storage.GetPR(ctx, prID)
}

// CreatePR creates PR and auto-assigns 1 reviewer (POST /pullRequest/create).
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
	if author.TeamID == uuid.Nil {
		return nil, fmt.Errorf("%s: author has no team", domain.ErrNotFound)
	}
	teamMembers, err := s.storage.GetUsersByTeamID(ctx, author.TeamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}
	reviewers := s.selectReviewers(teamMembers, author.UserID, 1)
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

// MergePR marks PR as MERGED (POST /pullRequest/merge) - only if all reviewers approved.
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
	if pr.Status == domain.StatusRejected {
		return nil, fmt.Errorf("%s: PR was rejected", domain.ErrPRRejected)
	}
	// Check if all reviewers approved
	if !s.allReviewersApproved(pr) {
		return nil, fmt.Errorf("%s: not all reviewers have approved", domain.ErrNotAllApproved)
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

// ApprovePR adds reviewer's approval to PR.
func (s *Service) ApprovePR(ctx context.Context, req *domain.ApprovePRRequest) (*domain.PullRequest, bool, error) {
	log := logger.FromContext(ctx)
	log.Info(ctx, "approving PR", zap.String("pr_id", req.PullRequestID), zap.String("reviewer_id", req.ReviewerID))

	pr, err := s.storage.GetPR(ctx, req.PullRequestID)
	if err != nil {
		return nil, false, fmt.Errorf("%s: PR not found", domain.ErrNotFound)
	}

	if pr.Status != domain.StatusOpen {
		return nil, false, fmt.Errorf("%s: PR is not open", domain.ErrPRNotOpen)
	}

	// Check if reviewer is assigned
	isAssigned := false
	for _, r := range pr.AssignedReviewers {
		if r == req.ReviewerID {
			isAssigned = true
			break
		}
	}
	if !isAssigned {
		return nil, false, fmt.Errorf("%s: reviewer is not assigned to this PR", domain.ErrNotAssigned)
	}

	// Check if already approved
	for _, a := range pr.ApprovedBy {
		if a == req.ReviewerID {
			// Already approved, return current state
			return pr, s.allReviewersApproved(pr), nil
		}
	}

	// Add approval
	pr.ApprovedBy = append(pr.ApprovedBy, req.ReviewerID)

	if err := s.storage.UpdatePR(ctx, pr); err != nil {
		log.Error(ctx, "failed to approve PR", zap.Error(err))
		return nil, false, err
	}

	allApproved := s.allReviewersApproved(pr)
	log.Info(ctx, "PR approved by reviewer",
		zap.String("pr_id", req.PullRequestID),
		zap.String("reviewer_id", req.ReviewerID),
		zap.Bool("all_approved", allApproved))

	return pr, allApproved, nil
}

// RejectPR marks PR as rejected.
func (s *Service) RejectPR(ctx context.Context, req *domain.RejectPRRequest) (*domain.PullRequest, error) {
	log := logger.FromContext(ctx)
	log.Info(ctx, "rejecting PR", zap.String("pr_id", req.PullRequestID), zap.String("reviewer_id", req.ReviewerID))

	pr, err := s.storage.GetPR(ctx, req.PullRequestID)
	if err != nil {
		return nil, fmt.Errorf("%s: PR not found", domain.ErrNotFound)
	}

	if pr.Status != domain.StatusOpen {
		return nil, fmt.Errorf("%s: PR is not open", domain.ErrPRNotOpen)
	}

	// Check if reviewer is assigned
	isAssigned := false
	for _, r := range pr.AssignedReviewers {
		if r == req.ReviewerID {
			isAssigned = true
			break
		}
	}
	if !isAssigned {
		return nil, fmt.Errorf("%s: reviewer is not assigned to this PR", domain.ErrNotAssigned)
	}

	pr.Status = domain.StatusRejected

	if err := s.storage.UpdatePR(ctx, pr); err != nil {
		log.Error(ctx, "failed to reject PR", zap.Error(err))
		return nil, err
	}

	log.Info(ctx, "PR rejected", zap.String("pr_id", req.PullRequestID), zap.String("reviewer_id", req.ReviewerID))
	return pr, nil
}

// allReviewersApproved checks if all assigned reviewers have approved.
func (s *Service) allReviewersApproved(pr *domain.PullRequest) bool {
	if len(pr.AssignedReviewers) == 0 {
		return true // No reviewers required
	}
	approvedSet := make(map[string]bool)
	for _, a := range pr.ApprovedBy {
		approvedSet[a] = true
	}
	for _, r := range pr.AssignedReviewers {
		if !approvedSet[r] {
			return false
		}
	}
	return true
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
	teamMembers, err := s.storage.GetUsersByTeamID(ctx, oldReviewer.TeamID)
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

// GetPRsByAuthor returns PRs authored by user.
func (s *Service) GetPRsByAuthor(ctx context.Context, authorID string) ([]*domain.PullRequestShort, error) {
	prs, err := s.storage.GetPRsByAuthor(ctx, authorID)
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

// GetStatistics returns various statistics about the system.
func (s *Service) GetStatistics(ctx context.Context) (*domain.StatisticsResponse, error) {
	log := logger.FromContext(ctx)
	log.Info(ctx, "fetching statistics")
	stats := &domain.StatisticsResponse{
		PRsByStatus: make(map[string]int),
	}
	// Get counts
	totalPRs, err := s.storage.GetTotalPRsCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get total PRs count: %w", err)
	}
	stats.TotalPRs = totalPRs
	openPRs, err := s.storage.GetPRsCountByStatus(ctx, domain.StatusOpen)
	if err != nil {
		return nil, fmt.Errorf("failed to get open PRs count: %w", err)
	}
	stats.OpenPRs = openPRs
	stats.PRsByStatus["OPEN"] = openPRs
	mergedPRs, err := s.storage.GetPRsCountByStatus(ctx, domain.StatusMerged)
	if err != nil {
		return nil, fmt.Errorf("failed to get merged PRs count: %w", err)
	}
	stats.MergedPRs = mergedPRs
	stats.PRsByStatus["MERGED"] = mergedPRs
	totalTeams, err := s.storage.GetTotalTeamsCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get total teams count: %w", err)
	}
	stats.TotalTeams = totalTeams
	totalUsers, err := s.storage.GetTotalUsersCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get total users count: %w", err)
	}
	stats.TotalUsers = totalUsers
	activeUsers, err := s.storage.GetActiveUsersCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active users count: %w", err)
	}
	stats.ActiveUsers = activeUsers
	// Get user assignment statistics
	users, err := s.storage.GetAllUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all users: %w", err)
	}
	allPRs, err := s.storage.GetAllPRs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all PRs: %w", err)
	}
	// Calculate assignments per user
	userAssignments := make(map[string]*domain.UserAssignmentStats)
	for _, user := range users {
		userAssignments[user.UserID] = &domain.UserAssignmentStats{
			UserID:           user.UserID,
			Username:         user.Username,
			TeamName:         user.TeamName,
			AssignedPRsCount: 0,
			OpenPRsCount:     0,
			MergedPRsCount:   0,
		}
	}
	for _, pr := range allPRs {
		for _, reviewerID := range pr.AssignedReviewers {
			if userStat, exists := userAssignments[reviewerID]; exists {
				userStat.AssignedPRsCount++
				if pr.Status == domain.StatusOpen {
					userStat.OpenPRsCount++
				} else if pr.Status == domain.StatusMerged {
					userStat.MergedPRsCount++
				}
			}
		}
	}
	// Convert map to slice
	stats.UserAssignments = make([]domain.UserAssignmentStats, 0, len(userAssignments))
	for _, userStat := range userAssignments {
		stats.UserAssignments = append(stats.UserAssignments, *userStat)
	}
	log.Info(ctx, "statistics fetched successfully",
		zap.Int("total_prs", stats.TotalPRs),
		zap.Int("total_users", stats.TotalUsers),
		zap.Int("total_teams", stats.TotalTeams),
	)
	return stats, nil
}

// BulkDeactivateTeamUsers deactivates all users in a team and reassigns their open PRs.
func (s *Service) BulkDeactivateTeamUsers(ctx context.Context, req *domain.BulkDeactivateRequest) (*domain.BulkDeactivateResponse, error) {
	log := logger.FromContext(ctx)
	log.Info(ctx, "bulk deactivating team users", zap.String("team_name", req.TeamName))
	// Get team members
	team, err := s.storage.GetTeam(ctx, req.TeamName)
	if err != nil {
		return nil, fmt.Errorf("%s: team not found", domain.ErrNotFound)
	}
	// Extract user IDs from team
	userIDs := make([]string, 0, len(team.Members))
	for _, member := range team.Members {
		if member.IsActive {
			userIDs = append(userIDs, member.UserID)
		}
	}
	if len(userIDs) == 0 {
		log.Info(ctx, "no active users to deactivate", zap.String("team_name", req.TeamName))
		return &domain.BulkDeactivateResponse{
			DeactivatedCount: 0,
			ReassignedPRs:    []domain.PRReassignmentSummary{},
		}, nil
	}
	// Get all open PRs assigned to these users
	openPRs, err := s.storage.GetOpenPRsByReviewers(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get open PRs: %w", err)
	}
	log.Info(ctx, "found open PRs to reassign",
		zap.Int("count", len(openPRs)),
		zap.Strings("deactivating_users", userIDs),
	)
	// Track reassignments
	reassignments := make([]domain.PRReassignmentSummary, 0)
	// Process each PR
	for _, pr := range openPRs {
		oldReviewers := make([]string, len(pr.AssignedReviewers))
		copy(oldReviewers, pr.AssignedReviewers)
		newReviewers := make([]string, 0, len(pr.AssignedReviewers))
		needsReassignment := false
		// Check which reviewers need to be replaced
		for _, reviewerID := range pr.AssignedReviewers {
			isDeactivating := false
			for _, uid := range userIDs {
				if reviewerID == uid {
					isDeactivating = true
					needsReassignment = true
					break
				}
			}
			if !isDeactivating {
				newReviewers = append(newReviewers, reviewerID)
			}
		}
		if !needsReassignment {
			continue
		}
		// Try to find replacement reviewers from author's team
		author, err := s.storage.GetUser(ctx, pr.AuthorID)
		if err != nil {
			log.Warn(ctx, "failed to get author for PR", zap.String("pr_id", pr.PullRequestID), zap.Error(err))
			pr.AssignedReviewers = newReviewers
			if err := s.storage.UpdatePR(ctx, pr); err != nil {
				log.Error(ctx, "failed to update PR reviewers", zap.String("pr_id", pr.PullRequestID), zap.Error(err))
			}
			continue
		}
		// Get potential reviewers from author's team (excluding deactivating users and author)
		teamMembers, err := s.storage.GetUsersByTeamID(ctx, author.TeamID)
		if err != nil {
			log.Warn(ctx, "failed to get team members", zap.String("team_id", author.TeamID.String()), zap.Error(err))
			pr.AssignedReviewers = newReviewers
			if err := s.storage.UpdatePR(ctx, pr); err != nil {
				log.Error(ctx, "failed to update PR reviewers", zap.String("pr_id", pr.PullRequestID), zap.Error(err))
			}
			continue
		}
		// Find available candidates (active, not author, not already assigned, not being deactivated)
		excludeMap := make(map[string]bool)
		excludeMap[pr.AuthorID] = true
		for _, rid := range newReviewers {
			excludeMap[rid] = true
		}
		for _, uid := range userIDs {
			excludeMap[uid] = true
		}
		var candidates []*domain.User
		for _, member := range teamMembers {
			if member.IsActive && !excludeMap[member.UserID] {
				candidates = append(candidates, member)
			}
		}
		// Assign new reviewers up to 2 total
		neededReviewers := 2 - len(newReviewers)
		if neededReviewers > 0 && len(candidates) > 0 {
			// Shuffle and pick
			rand.Shuffle(len(candidates), func(i, j int) {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			})
			count := min(len(candidates), neededReviewers)
			for i := 0; i < count; i++ {
				newReviewers = append(newReviewers, candidates[i].UserID)
			}
		}
		// Update PR with new reviewers
		pr.AssignedReviewers = newReviewers
		if err := s.storage.UpdatePR(ctx, pr); err != nil {
			log.Error(ctx, "failed to update PR", zap.String("pr_id", pr.PullRequestID), zap.Error(err))
			continue
		}
		reassignments = append(reassignments, domain.PRReassignmentSummary{
			PullRequestID: pr.PullRequestID,
			OldReviewers:  oldReviewers,
			NewReviewers:  newReviewers,
		})
		log.Info(ctx, "PR reviewers reassigned",
			zap.String("pr_id", pr.PullRequestID),
			zap.Strings("old_reviewers", oldReviewers),
			zap.Strings("new_reviewers", newReviewers),
		)
	}
	// Deactivate all users in the team
	if err := s.storage.BulkUpdateUsersActive(ctx, userIDs, false); err != nil {
		return nil, fmt.Errorf("failed to deactivate users: %w", err)
	}
	log.Info(ctx, "bulk deactivation completed",
		zap.Int("deactivated_count", len(userIDs)),
		zap.Int("reassigned_prs", len(reassignments)),
	)
	return &domain.BulkDeactivateResponse{
		DeactivatedCount: len(userIDs),
		ReassignedPRs:    reassignments,
	}, nil
}

// GetTeamIDByName resolves team name to team UUID.
func (s *Service) GetTeamIDByName(ctx context.Context, teamName string) (string, error) {
	log := logger.FromContext(ctx)
	log.Info(ctx, "resolving team name to ID", zap.String("team_name", teamName))

	teamID, err := s.storage.GetTeamIDByName(ctx, teamName)
	if err != nil {
		return "", err
	}

	return teamID.String(), nil
}

// GetUser returns user by ID.
func (s *Service) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	log := logger.FromContext(ctx)
	log.Info(ctx, "getting user", zap.String("user_id", userID))
	return s.storage.GetUser(ctx, userID)
}
