package service

import (
	"context"
	"errors"

	"github.com/Meldy183/code-storage-service/internal/domain"
	"github.com/Meldy183/code-storage-service/internal/storage"
	"github.com/Meldy183/shared/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service provides business logic for code storage operations
type Service struct {
	storage storage.Storage
}

// NewService creates a new Service instance
func NewService(s storage.Storage) *Service {
	return &Service{storage: s}
}

// InitRepository initializes a new repository for a team with the initial code
func (s *Service) InitRepository(ctx context.Context, teamID uuid.UUID, commitName string, code []byte) (*domain.Commit, error) {
	log := logger.FromContext(ctx)

	// Check if team exists
	exists, err := s.storage.TeamExists(ctx, teamID)
	if err != nil {
		log.Error(ctx, "failed to check team existence", zap.Error(err))
		return nil, err
	}
	if !exists {
		return nil, domain.ErrTeamNotFound
	}

	// Create root commit
	commit, err := s.storage.InitRepository(ctx, teamID, commitName, code)
	if err != nil {
		log.Error(ctx, "failed to initialize repository", zap.Error(err))
		return nil, err
	}

	log.Info(ctx, "repository initialized",
		zap.String("team_id", teamID.String()),
		zap.String("root_commit", commit.ID.String()),
		zap.String("commit_name", commitName),
	)

	return commit, nil
}

// Push creates a new commit on top of an existing parent commit
func (s *Service) Push(ctx context.Context, teamID, rootCommit, parentCommitID uuid.UUID, commitName string, code []byte) (*domain.Commit, error) {
	log := logger.FromContext(ctx)

	// Check if team exists
	exists, err := s.storage.TeamExists(ctx, teamID)
	if err != nil {
		log.Error(ctx, "failed to check team existence", zap.Error(err))
		return nil, err
	}
	if !exists {
		return nil, domain.ErrTeamNotFound
	}

	// Check if root commit exists
	rootExists, err := s.storage.RootCommitExists(ctx, teamID, rootCommit)
	if err != nil {
		log.Error(ctx, "failed to check root commit existence", zap.Error(err))
		return nil, err
	}
	if !rootExists {
		return nil, domain.ErrRootCommitNotFound
	}

	// Check if parent commit exists
	_, err = s.storage.GetCommit(ctx, teamID, rootCommit, parentCommitID)
	if err != nil {
		if errors.Is(err, domain.ErrCommitNotFound) {
			return nil, domain.ErrInvalidParent
		}
		log.Error(ctx, "failed to get parent commit", zap.Error(err))
		return nil, err
	}

	// Create new commit
	commit, err := s.storage.CreateCommit(ctx, teamID, rootCommit, parentCommitID, commitName, code)
	if err != nil {
		log.Error(ctx, "failed to create commit", zap.Error(err))
		return nil, err
	}

	log.Info(ctx, "commit created",
		zap.String("commit_id", commit.ID.String()),
		zap.String("commit_name", commitName),
		zap.String("parent_id", parentCommitID.String()),
	)

	return commit, nil
}

// Checkout retrieves the code from a specific commit
func (s *Service) Checkout(ctx context.Context, teamID, rootCommit, commitID uuid.UUID) ([]byte, error) {
	log := logger.FromContext(ctx)

	// Check if team exists
	exists, err := s.storage.TeamExists(ctx, teamID)
	if err != nil {
		log.Error(ctx, "failed to check team existence", zap.Error(err))
		return nil, err
	}
	if !exists {
		return nil, domain.ErrTeamNotFound
	}

	// Check if root commit exists
	rootExists, err := s.storage.RootCommitExists(ctx, teamID, rootCommit)
	if err != nil {
		log.Error(ctx, "failed to check root commit existence", zap.Error(err))
		return nil, err
	}
	if !rootExists {
		return nil, domain.ErrRootCommitNotFound
	}

	// Get commit code
	code, err := s.storage.GetCommitCode(ctx, teamID, rootCommit, commitID)
	if err != nil {
		if errors.Is(err, domain.ErrCommitNotFound) {
			return nil, domain.ErrCommitNotFound
		}
		log.Error(ctx, "failed to get commit code", zap.Error(err))
		return nil, err
	}

	log.Info(ctx, "checkout successful",
		zap.String("commit_id", commitID.String()),
	)

	return code, nil
}

// Merge merges two leaf commits into a new merge commit
func (s *Service) Merge(ctx context.Context, teamID, rootCommit, commitID1, commitID2 uuid.UUID) (*domain.Commit, error) {
	log := logger.FromContext(ctx)

	// Check if team exists
	exists, err := s.storage.TeamExists(ctx, teamID)
	if err != nil {
		log.Error(ctx, "failed to check team existence", zap.Error(err))
		return nil, err
	}
	if !exists {
		return nil, domain.ErrTeamNotFound
	}

	// Check if root commit exists
	rootExists, err := s.storage.RootCommitExists(ctx, teamID, rootCommit)
	if err != nil {
		log.Error(ctx, "failed to check root commit existence", zap.Error(err))
		return nil, err
	}
	if !rootExists {
		return nil, domain.ErrRootCommitNotFound
	}

	// Check if both commits exist
	_, err = s.storage.GetCommit(ctx, teamID, rootCommit, commitID1)
	if err != nil {
		if errors.Is(err, domain.ErrCommitNotFound) {
			return nil, domain.ErrCommitNotFound
		}
		log.Error(ctx, "failed to get commit1", zap.Error(err))
		return nil, err
	}

	_, err = s.storage.GetCommit(ctx, teamID, rootCommit, commitID2)
	if err != nil {
		if errors.Is(err, domain.ErrCommitNotFound) {
			return nil, domain.ErrCommitNotFound
		}
		log.Error(ctx, "failed to get commit2", zap.Error(err))
		return nil, err
	}

	// Check if both commits are leaf commits
	isLeaf1, err := s.storage.IsLeafCommit(ctx, teamID, rootCommit, commitID1)
	if err != nil {
		log.Error(ctx, "failed to check if commit1 is leaf", zap.Error(err))
		return nil, err
	}
	if !isLeaf1 {
		return nil, domain.ErrCommitNotLeaf
	}

	isLeaf2, err := s.storage.IsLeafCommit(ctx, teamID, rootCommit, commitID2)
	if err != nil {
		log.Error(ctx, "failed to check if commit2 is leaf", zap.Error(err))
		return nil, err
	}
	if !isLeaf2 {
		return nil, domain.ErrCommitNotLeaf
	}

	// Create merge commit
	commit, err := s.storage.MergeCommits(ctx, teamID, rootCommit, commitID1, commitID2)
	if err != nil {
		log.Error(ctx, "failed to create merge commit", zap.Error(err))
		return nil, err
	}

	log.Info(ctx, "merge commit created",
		zap.String("commit_id", commit.ID.String()),
		zap.String("parent1", commitID1.String()),
		zap.String("parent2", commitID2.String()),
	)

	return commit, nil
}

// GetCommitName retrieves the name of a commit
func (s *Service) GetCommitName(ctx context.Context, commitID uuid.UUID) (string, error) {
	log := logger.FromContext(ctx)

	name, err := s.storage.GetCommitName(ctx, commitID)
	if err != nil {
		if errors.Is(err, domain.ErrCommitNotFound) {
			return "", domain.ErrCommitNotFound
		}
		log.Error(ctx, "failed to get commit name", zap.Error(err))
		return "", err
	}

	return name, nil
}

// ListCommits returns all commits for a repository
func (s *Service) ListCommits(ctx context.Context, teamID, rootCommit uuid.UUID) ([]*domain.Commit, error) {
	log := logger.FromContext(ctx)

	// Check if team exists
	exists, err := s.storage.TeamExists(ctx, teamID)
	if err != nil {
		log.Error(ctx, "failed to check team existence", zap.Error(err))
		return nil, err
	}
	if !exists {
		return nil, domain.ErrTeamNotFound
	}

	commits, err := s.storage.ListCommits(ctx, teamID, rootCommit)
	if err != nil {
		log.Error(ctx, "failed to list commits", zap.Error(err))
		return nil, err
	}

	return commits, nil
}

// GetCommitIDByName retrieves commit ID by its name within a repository
func (s *Service) GetCommitIDByName(ctx context.Context, teamID, rootCommit uuid.UUID, name string) (uuid.UUID, error) {
	log := logger.FromContext(ctx)

	// Check if team exists
	exists, err := s.storage.TeamExists(ctx, teamID)
	if err != nil {
		log.Error(ctx, "failed to check team existence", zap.Error(err))
		return uuid.Nil, err
	}
	if !exists {
		return uuid.Nil, domain.ErrTeamNotFound
	}

	// Check if root commit exists
	rootExists, err := s.storage.RootCommitExists(ctx, teamID, rootCommit)
	if err != nil {
		log.Error(ctx, "failed to check root commit existence", zap.Error(err))
		return uuid.Nil, err
	}
	if !rootExists {
		return uuid.Nil, domain.ErrRootCommitNotFound
	}

	commitID, err := s.storage.GetCommitIDByName(ctx, teamID, rootCommit, name)
	if err != nil {
		if errors.Is(err, domain.ErrCommitNotFound) {
			return uuid.Nil, domain.ErrCommitNotFound
		}
		log.Error(ctx, "failed to get commit by name", zap.Error(err))
		return uuid.Nil, err
	}

	return commitID, nil
}

// GetRootCommitByRepoName finds the root commit for a repository by its name
func (s *Service) GetRootCommitByRepoName(ctx context.Context, teamID uuid.UUID, repoName string) (uuid.UUID, error) {
	log := logger.FromContext(ctx)

	// Check if team exists
	exists, err := s.storage.TeamExists(ctx, teamID)
	if err != nil {
		log.Error(ctx, "failed to check team existence", zap.Error(err))
		return uuid.Nil, err
	}
	if !exists {
		return uuid.Nil, domain.ErrTeamNotFound
	}

	rootCommit, err := s.storage.GetRootCommitByRepoName(ctx, teamID, repoName)
	if err != nil {
		if errors.Is(err, domain.ErrCommitNotFound) {
			return uuid.Nil, domain.ErrCommitNotFound
		}
		log.Error(ctx, "failed to get root commit by repo name", zap.Error(err))
		return uuid.Nil, err
	}

	return rootCommit, nil
}
