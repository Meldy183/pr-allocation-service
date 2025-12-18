package service

import (
	"context"
	"fmt"
	"strings"

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
	// In-memory storage for repo -> root_commit mapping
	repoRootCommits map[string]uuid.UUID // key: "teamID:repoName"
}

// PRMetadata stores additional PR info not in pr-allocation-service
type PRMetadata struct {
	PRName           string
	TeamName         string
	RepoName         string
	RootCommit       uuid.UUID
	SourceCommit     uuid.UUID
	SourceCommitName string
	TargetCommit     uuid.UUID
	TargetCommitName string
	TeamID           uuid.UUID
}

// NewService creates a new Service instance
func NewService(prClient *client.PRAllocationClient, codeClient *client.CodeStorageClient) *Service {
	return &Service{
		prClient:        prClient,
		codeClient:      codeClient,
		prMetadata:      make(map[string]*PRMetadata),
		repoRootCommits: make(map[string]uuid.UUID),
	}
}

// CreateTeam creates a new team
func (s *Service) CreateTeam(ctx context.Context, req *domain.CreateTeamRequest) (*domain.Team, error) {
	log := logger.FromContext(ctx)
	log.Info(ctx, "creating team", zap.String("team_name", req.TeamName))

	// Convert domain members to client members (username only, IDs are generated)
	members := make([]client.TeamMember, len(req.Members))
	for i, m := range req.Members {
		members[i] = client.TeamMember{
			UserID:   m.Username, // Use username as user_id for simplicity
			Username: m.Username,
			IsActive: m.IsActive,
		}
	}

	// Create team via pr-allocation-service
	team, err := s.prClient.CreateTeam(ctx, req.TeamName, members)
	if err != nil {
		log.Error(ctx, "failed to create team", zap.Error(err))
		if strings.Contains(err.Error(), "already exists") {
			return nil, domain.ErrTeamExists
		}
		return nil, fmt.Errorf("failed to create team: %w", err)
	}

	// Convert response to domain (response includes IDs)
	domainMembers := make([]domain.TeamMemberResponse, len(team.Members))
	for i, m := range team.Members {
		domainMembers[i] = domain.TeamMemberResponse{
			UserID:   m.UserID,
			Username: m.Username,
			IsActive: m.IsActive,
		}
	}

	log.Info(ctx, "team created", zap.String("team_id", team.TeamID.String()), zap.String("team_name", team.TeamName))
	return &domain.Team{
		TeamID:   team.TeamID,
		TeamName: team.TeamName,
		Members:  domainMembers,
	}, nil
}

// GetTeam gets a team by name
func (s *Service) GetTeam(ctx context.Context, teamName string) (*domain.Team, error) {
	log := logger.FromContext(ctx)
	log.Info(ctx, "getting team", zap.String("team_name", teamName))

	team, err := s.prClient.GetTeam(ctx, teamName)
	if err != nil {
		log.Error(ctx, "failed to get team", zap.Error(err))
		return nil, domain.ErrTeamNotFound
	}

	// Convert response to domain
	domainMembers := make([]domain.TeamMemberResponse, len(team.Members))
	for i, m := range team.Members {
		domainMembers[i] = domain.TeamMemberResponse{
			UserID:   m.UserID,
			Username: m.Username,
			IsActive: m.IsActive,
		}
	}

	return &domain.Team{
		TeamID:   team.TeamID,
		TeamName: team.TeamName,
		Members:  domainMembers,
	}, nil
}

// GetUserProfile gets user profile by username
func (s *Service) GetUserProfile(ctx context.Context, username string) (*domain.UserProfile, error) {
	log := logger.FromContext(ctx)

	// Get user from pr-allocation-service (by username)
	user, err := s.prClient.GetUser(ctx, username)
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

// InitRepository initializes a new repository using names
// InitRepository initializes a new repository using names
func (s *Service) InitRepository(ctx context.Context, username, teamName, repoName, commitName string, code []byte) (*domain.Commit, error) {
	log := logger.FromContext(ctx)

	// Resolve team name to UUID
	teamID, err := s.prClient.ResolveTeamID(ctx, teamName)
	if err != nil {
		log.Error(ctx, "failed to resolve team", zap.Error(err))
		return nil, domain.ErrTeamNotFound
	}

	// Verify user belongs to team
	if err := s.verifyUserAccess(ctx, username, teamName); err != nil {
		return nil, err
	}

	// Initialize repository in code-storage with commit name
	commit, err := s.codeClient.InitRepositoryWithName(ctx, teamID, commitName, code)
	if err != nil {
		log.Error(ctx, "failed to init repository", zap.Error(err))
		if strings.Contains(err.Error(), "not found") {
			return nil, domain.ErrTeamNotFound
		}
		return nil, fmt.Errorf("failed to init repository: %w", err)
	}

	// Save repo -> root_commit mapping
	repoKey := fmt.Sprintf("%s:%s", teamID.String(), repoName)
	s.repoRootCommits[repoKey] = commit.RootCommit

	log.Info(ctx, "repository initialized",
		zap.String("username", username),
		zap.String("repo_name", repoName),
		zap.String("commit_name", commitName),
		zap.String("root_commit", commit.RootCommit.String()),
	)

	return &domain.Commit{
		CommitID:        commit.CommitID,
		RootCommit:      commit.RootCommit,
		ParentCommitIDs: commit.ParentCommitIDs,
		CommitName:      &commitName,
		RepoName:        &repoName,
		CreatedAt:       commit.CreatedAt,
	}, nil
}

// Push creates a new commit using names
func (s *Service) Push(ctx context.Context, username, teamName, repoName, parentCommitName, commitName string, code []byte) (*domain.Commit, error) {
	log := logger.FromContext(ctx)

	// Verify user access
	if err := s.verifyUserAccess(ctx, username, teamName); err != nil {
		return nil, err
	}

	// Resolve team name to UUID
	teamID, err := s.prClient.ResolveTeamID(ctx, teamName)
	if err != nil {
		log.Error(ctx, "failed to resolve team", zap.Error(err))
		return nil, domain.ErrTeamNotFound
	}

	// Get root commit from cache
	rootCommit, ok := s.getRootCommit(teamID, repoName)
	if !ok {
		log.Error(ctx, "repository not found in cache", zap.String("repo", repoName))
		return nil, domain.ErrCommitNotFound
	}

	// Resolve parent commit by name
	parentCommit, err := s.resolveCommitByName(ctx, teamID, repoName, parentCommitName)
	if err != nil {
		log.Error(ctx, "failed to resolve parent commit", zap.Error(err))
		return nil, domain.ErrCommitNotFound
	}

	// Push commit with name
	commit, err := s.codeClient.PushWithName(ctx, teamID, rootCommit, parentCommit, commitName, code)
	if err != nil {
		log.Error(ctx, "failed to push commit", zap.Error(err))
		if strings.Contains(err.Error(), "not found") {
			return nil, domain.ErrCommitNotFound
		}
		return nil, fmt.Errorf("failed to push: %w", err)
	}

	log.Info(ctx, "commit pushed",
		zap.String("username", username),
		zap.String("commit_name", commitName),
		zap.String("commit_id", commit.CommitID.String()),
	)

	return &domain.Commit{
		CommitID:        commit.CommitID,
		RootCommit:      commit.RootCommit,
		ParentCommitIDs: commit.ParentCommitIDs,
		CommitName:      &commitName,
		RepoName:        &repoName,
		CreatedAt:       commit.CreatedAt,
	}, nil
}

// Checkout retrieves code for a commit using names
func (s *Service) Checkout(ctx context.Context, username, teamName, repoName, commitName string) ([]byte, error) {
	log := logger.FromContext(ctx)

	// Verify user access
	if err := s.verifyUserAccess(ctx, username, teamName); err != nil {
		return nil, err
	}

	// Resolve team name to UUID
	teamID, err := s.prClient.ResolveTeamID(ctx, teamName)
	if err != nil {
		log.Error(ctx, "failed to resolve team", zap.Error(err))
		return nil, domain.ErrTeamNotFound
	}

	// Get root commit from cache
	rootCommit, ok := s.getRootCommit(teamID, repoName)
	if !ok {
		log.Error(ctx, "repository not found in cache", zap.String("repo", repoName))
		return nil, domain.ErrCommitNotFound
	}

	// Resolve commit by name
	commitID, err := s.resolveCommitByName(ctx, teamID, repoName, commitName)
	if err != nil {
		log.Error(ctx, "failed to resolve commit", zap.Error(err))
		return nil, domain.ErrCommitNotFound
	}

	code, err := s.codeClient.Checkout(ctx, teamID, rootCommit, commitID)
	if err != nil {
		log.Error(ctx, "failed to checkout", zap.Error(err))
		if strings.Contains(err.Error(), "not found") {
			return nil, domain.ErrCommitNotFound
		}
		return nil, fmt.Errorf("failed to checkout: %w", err)
	}

	log.Info(ctx, "checkout successful",
		zap.String("username", username),
		zap.String("commit_name", commitName),
	)

	return code, nil
}

// ListCommits lists all commits for a repository using names
func (s *Service) ListCommits(ctx context.Context, username, teamName, repoName string) ([]domain.Commit, error) {
	log := logger.FromContext(ctx)

	// Verify user access
	if err := s.verifyUserAccess(ctx, username, teamName); err != nil {
		return nil, err
	}

	// Resolve team name to UUID
	teamID, err := s.prClient.ResolveTeamID(ctx, teamName)
	if err != nil {
		log.Error(ctx, "failed to resolve team", zap.Error(err))
		return nil, domain.ErrTeamNotFound
	}

	// Get root commit from cache
	rootCommit, ok := s.getRootCommit(teamID, repoName)
	if !ok {
		log.Error(ctx, "repository not found in cache", zap.String("repo", repoName))
		return nil, domain.ErrCommitNotFound
	}

	// List commits from code-storage
	commits, err := s.codeClient.ListCommits(ctx, teamID, rootCommit)
	if err != nil {
		log.Error(ctx, "failed to list commits", zap.Error(err))
		return nil, fmt.Errorf("failed to list commits: %w", err)
	}

	// Convert to domain commits
	result := make([]domain.Commit, len(commits))
	for i, c := range commits {
		commitName := ""
		if c.CommitName != nil {
			commitName = *c.CommitName
		}
		result[i] = domain.Commit{
			CommitID:        c.CommitID,
			RootCommit:      c.RootCommit,
			ParentCommitIDs: c.ParentCommitIDs,
			CommitName:      &commitName,
			RepoName:        &repoName,
			CreatedAt:       c.CreatedAt,
		}
		// Mark root commit
		if c.CommitID == rootCommit {
			result[i].CommitName = &commitName
		}
	}

	log.Info(ctx, "listed commits",
		zap.String("username", username),
		zap.String("repo", repoName),
		zap.Int("count", len(result)),
	)

	return result, nil
}

// CreatePR creates a new pull request using names
func (s *Service) CreatePR(ctx context.Context, username string, req *domain.CreatePRRequest) (*domain.PullRequest, error) {
	log := logger.FromContext(ctx)

	// Verify user access
	if err := s.verifyUserAccess(ctx, username, req.TeamName); err != nil {
		return nil, err
	}

	// Resolve team name to UUID
	teamID, err := s.prClient.ResolveTeamID(ctx, req.TeamName)
	if err != nil {
		log.Error(ctx, "failed to resolve team", zap.Error(err))
		return nil, domain.ErrTeamNotFound
	}

	// Get root commit from cache
	rootCommit, ok := s.getRootCommit(teamID, req.RepoName)
	if !ok {
		log.Error(ctx, "repository not found in cache", zap.String("repo", req.RepoName))
		return nil, domain.ErrCommitNotFound
	}

	// Resolve source commit by name
	sourceCommit, err := s.resolveCommitByName(ctx, teamID, req.RepoName, req.SourceCommitName)
	if err != nil {
		log.Error(ctx, "failed to resolve source commit", zap.Error(err))
		return nil, domain.ErrCommitNotFound
	}

	// Resolve target commit by name
	targetCommit, err := s.resolveCommitByName(ctx, teamID, req.RepoName, req.TargetCommitName)
	if err != nil {
		log.Error(ctx, "failed to resolve target commit", zap.Error(err))
		return nil, domain.ErrCommitNotFound
	}

	// Generate PR ID
	prID := fmt.Sprintf("pr-%s", uuid.New().String()[:8])

	// Create PR in pr-allocation-service
	prResp, err := s.prClient.CreatePR(ctx, prID, req.Title, username)
	if err != nil {
		log.Error(ctx, "failed to create PR", zap.Error(err))
		if strings.Contains(err.Error(), "already exists") {
			return nil, domain.ErrPRAlreadyExists
		}
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}

	// Store metadata for later use (keyed by team_name:pr_name)
	metaKey := fmt.Sprintf("%s:%s", req.TeamName, req.PRName)
	s.prMetadata[metaKey] = &PRMetadata{
		PRName:           req.PRName,
		TeamName:         req.TeamName,
		RepoName:         req.RepoName,
		RootCommit:       rootCommit,
		SourceCommit:     sourceCommit,
		SourceCommitName: req.SourceCommitName,
		TargetCommit:     targetCommit,
		TargetCommitName: req.TargetCommitName,
		TeamID:           teamID,
	}
	// Also store by prID for internal lookups
	s.prMetadata[prID] = s.prMetadata[metaKey]

	log.Info(ctx, "PR created",
		zap.String("pr_name", req.PRName),
		zap.String("author", username),
		zap.Strings("reviewers", prResp.AssignedReviewers),
	)

	return &domain.PullRequest{
		PRID:             prResp.PRID,
		PRName:           req.PRName,
		Title:            req.Title,
		AuthorID:         prResp.AuthorID,
		AuthorName:       username,
		Status:           prResp.Status,
		ReviewerIDs:      prResp.AssignedReviewers,
		SourceCommitID:   sourceCommit,
		SourceCommitName: req.SourceCommitName,
		TargetCommitID:   targetCommit,
		TargetCommitName: req.TargetCommitName,
		RootCommitID:     rootCommit,
		RepoName:         req.RepoName,
		TeamName:         req.TeamName,
		CreatedAt:        prResp.CreatedAt,
	}, nil
}

// GetMyPRs gets PRs authored by user
func (s *Service) GetMyPRs(ctx context.Context, username string, status string) ([]domain.PullRequest, error) {
	log := logger.FromContext(ctx)

	prs, err := s.prClient.GetPRsByAuthor(ctx, username)
	if err != nil {
		log.Error(ctx, "failed to get authored PRs", zap.Error(err))
		return nil, fmt.Errorf("failed to get authored PRs: %w", err)
	}

	result := make([]domain.PullRequest, 0, len(prs))
	for _, pr := range prs {
		if status != "" && pr.Status != status {
			continue
		}

		domainPR := domain.PullRequest{
			PRID:        pr.PRID,
			PRName:      pr.PRName,
			Title:       pr.PRName,
			AuthorID:    pr.AuthorID,
			AuthorName:  pr.AuthorID, // username = user_id
			Status:      pr.Status,
			ReviewerIDs: pr.AssignedReviewers,
			CreatedAt:   pr.CreatedAt,
			MergedAt:    pr.MergedAt,
		}

		// Add metadata if available
		if meta, ok := s.prMetadata[pr.PRID]; ok {
			domainPR.PRName = meta.PRName
			domainPR.TeamName = meta.TeamName
			domainPR.RepoName = meta.RepoName
			domainPR.RootCommitID = meta.RootCommit
			domainPR.SourceCommitID = meta.SourceCommit
			domainPR.SourceCommitName = meta.SourceCommitName
			domainPR.TargetCommitID = meta.TargetCommit
			domainPR.TargetCommitName = meta.TargetCommitName
		}

		result = append(result, domainPR)
	}

	return result, nil
}

// GetReviewPRs gets PRs where user is reviewer
func (s *Service) GetReviewPRs(ctx context.Context, username string, status string) ([]domain.PullRequest, error) {
	log := logger.FromContext(ctx)

	prs, err := s.prClient.GetPRsByReviewer(ctx, username)
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
			domainPR.PRName = meta.PRName
			domainPR.TeamName = meta.TeamName
			domainPR.RepoName = meta.RepoName
			domainPR.RootCommitID = meta.RootCommit
			domainPR.SourceCommitID = meta.SourceCommit
			domainPR.SourceCommitName = meta.SourceCommitName
			domainPR.TargetCommitID = meta.TargetCommit
			domainPR.TargetCommitName = meta.TargetCommitName
		}

		result = append(result, domainPR)
	}

	return result, nil
}

// ApprovePR approves a PR and triggers merge if all approved using names
func (s *Service) ApprovePR(ctx context.Context, username, teamName, prName string) (*domain.PullRequest, *domain.Commit, error) {
	log := logger.FromContext(ctx)

	// Get PR metadata by name
	metaKey := fmt.Sprintf("%s:%s", teamName, prName)
	meta, ok := s.prMetadata[metaKey]
	if !ok {
		return nil, nil, domain.ErrPRNotFound
	}

	// Verify user has access to this team
	if err := s.verifyUserAccess(ctx, username, teamName); err != nil {
		return nil, nil, err
	}

	// Find PR ID for pr-allocation-service
	var prID string
	for key, m := range s.prMetadata {
		if m == meta && strings.HasPrefix(key, "pr-") {
			prID = key
			break
		}
	}
	if prID == "" {
		return nil, nil, domain.ErrPRNotFound
	}

	// Approve PR in pr-allocation-service
	prResp, allApproved, err := s.prClient.ApprovePR(ctx, prID, username)
	if err != nil {
		log.Error(ctx, "failed to approve PR", zap.Error(err))
		return nil, nil, fmt.Errorf("failed to approve PR: %w", err)
	}

	log.Info(ctx, "PR approved",
		zap.String("pr_name", prName),
		zap.String("reviewer", username),
		zap.Bool("all_approved", allApproved),
	)

	pr := &domain.PullRequest{
		PRID:             prResp.PRID,
		PRName:           prName,
		Title:            prResp.PRName,
		AuthorID:         prResp.AuthorID,
		Status:           prResp.Status,
		ReviewerIDs:      prResp.AssignedReviewers,
		SourceCommitID:   meta.SourceCommit,
		SourceCommitName: meta.SourceCommitName,
		TargetCommitID:   meta.TargetCommit,
		TargetCommitName: meta.TargetCommitName,
		RootCommitID:     meta.RootCommit,
		RepoName:         meta.RepoName,
		TeamName:         meta.TeamName,
		CreatedAt:        prResp.CreatedAt,
		MergedAt:         prResp.MergedAt,
	}

	// If all reviewers approved, merge the code
	if allApproved {
		mergeCommit, err := s.codeClient.Merge(ctx, meta.TeamID, meta.RootCommit, meta.SourceCommit, meta.TargetCommit)
		if err != nil {
			log.Error(ctx, "failed to merge commits", zap.Error(err))
			return nil, nil, fmt.Errorf("failed to merge: %w", err)
		}

		// Mark PR as merged in pr-allocation-service
		prResp, err = s.prClient.MergePR(ctx, prID)
		if err != nil {
			log.Error(ctx, "failed to mark PR as merged", zap.Error(err))
			return nil, nil, fmt.Errorf("failed to update PR status: %w", err)
		}

		pr.Status = prResp.Status
		pr.MergedAt = prResp.MergedAt

		log.Info(ctx, "PR merged",
			zap.String("pr_name", prName),
			zap.String("merge_commit", mergeCommit.CommitID.String()),
		)

		commit := &domain.Commit{
			CommitID:        mergeCommit.CommitID,
			RootCommit:      mergeCommit.RootCommit,
			ParentCommitIDs: mergeCommit.ParentCommitIDs,
			RepoName:        &meta.RepoName,
			CreatedAt:       mergeCommit.CreatedAt,
		}

		return pr, commit, nil
	}

	return pr, nil, nil
}

// RejectPR rejects a PR using names
func (s *Service) RejectPR(ctx context.Context, username, teamName, prName string, reason string) (*domain.PullRequest, error) {
	log := logger.FromContext(ctx)

	// Get PR metadata by name
	metaKey := fmt.Sprintf("%s:%s", teamName, prName)
	meta, ok := s.prMetadata[metaKey]
	if !ok {
		return nil, domain.ErrPRNotFound
	}

	// Verify user access
	if err := s.verifyUserAccess(ctx, username, teamName); err != nil {
		return nil, err
	}

	// Find PR ID
	var prID string
	for key, m := range s.prMetadata {
		if m == meta && strings.HasPrefix(key, "pr-") {
			prID = key
			break
		}
	}
	if prID == "" {
		return nil, domain.ErrPRNotFound
	}

	// Reject PR in pr-allocation-service
	prResp, err := s.prClient.RejectPR(ctx, prID, username, reason)
	if err != nil {
		log.Error(ctx, "failed to reject PR", zap.Error(err))
		return nil, fmt.Errorf("failed to reject PR: %w", err)
	}

	log.Info(ctx, "PR rejected",
		zap.String("pr_name", prName),
		zap.String("reviewer", username),
		zap.String("reason", reason),
	)

	return &domain.PullRequest{
		PRID:             prResp.PRID,
		PRName:           prName,
		Title:            prResp.PRName,
		AuthorID:         prResp.AuthorID,
		Status:           prResp.Status,
		ReviewerIDs:      prResp.AssignedReviewers,
		SourceCommitID:   meta.SourceCommit,
		SourceCommitName: meta.SourceCommitName,
		TargetCommitID:   meta.TargetCommit,
		TargetCommitName: meta.TargetCommitName,
		RootCommitID:     meta.RootCommit,
		RepoName:         meta.RepoName,
		TeamName:         meta.TeamName,
	}, nil
}

// GetPRCode gets code for a PR using names
func (s *Service) GetPRCode(ctx context.Context, username, teamName, prName string) ([]byte, error) {
	log := logger.FromContext(ctx)

	// Verify user access
	if err := s.verifyUserAccess(ctx, username, teamName); err != nil {
		return nil, err
	}

	// Get PR metadata by name
	metaKey := fmt.Sprintf("%s:%s", teamName, prName)
	meta, ok := s.prMetadata[metaKey]
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

// verifyUserAccess checks if user belongs to the team
func (s *Service) verifyUserAccess(ctx context.Context, username, teamName string) error {
	user, err := s.prClient.GetUser(ctx, username)
	if err != nil {
		return domain.ErrUserNotFound
	}

	if !user.IsActive {
		return domain.ErrUserInactive
	}

	if user.TeamName != teamName {
		return domain.ErrAccessDenied
	}

	return nil
}

// getRootCommit gets root commit UUID for a repo from cache
func (s *Service) getRootCommit(teamID uuid.UUID, repoName string) (uuid.UUID, bool) {
	repoKey := fmt.Sprintf("%s:%s", teamID.String(), repoName)
	rootCommit, ok := s.repoRootCommits[repoKey]
	return rootCommit, ok
}

// resolveCommitByName resolves a commit name to UUID using code-storage
func (s *Service) resolveCommitByName(ctx context.Context, teamID uuid.UUID, repoName, commitName string) (uuid.UUID, error) {
	// First get root commit from cache
	rootCommit, ok := s.getRootCommit(teamID, repoName)
	if !ok {
		return uuid.Nil, fmt.Errorf("repository not found: %s", repoName)
	}

	// If commitName equals repoName, it's the root commit itself
	if commitName == repoName {
		return rootCommit, nil
	}

	// Otherwise resolve via code-storage
	return s.codeClient.GetCommitIDByName(ctx, teamID, rootCommit, commitName)
}
