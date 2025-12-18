package storage

import (
	"context"

	"github.com/Meldy183/code-storage-service/internal/domain"
	"github.com/google/uuid"
)

// Storage defines the interface for commit storage operations
type Storage interface {
	// Team operations
	TeamExists(ctx context.Context, teamID uuid.UUID) (bool, error)

	// Repository/Commit operations
	InitRepository(ctx context.Context, teamID uuid.UUID, commitName string, code []byte) (*domain.Commit, error)
	GetCommit(ctx context.Context, teamID, rootCommit, commitID uuid.UUID) (*domain.Commit, error)
	GetCommitCode(ctx context.Context, teamID, rootCommit, commitID uuid.UUID) ([]byte, error)
	CreateCommit(ctx context.Context, teamID, rootCommit, parentID uuid.UUID, commitName string, code []byte) (*domain.Commit, error)
	MergeCommits(ctx context.Context, teamID, rootCommit, commitID1, commitID2 uuid.UUID) (*domain.Commit, error)
	IsLeafCommit(ctx context.Context, teamID, rootCommit, commitID uuid.UUID) (bool, error)
	RootCommitExists(ctx context.Context, teamID, rootCommit uuid.UUID) (bool, error)

	// Commit name operations
	GetCommitName(ctx context.Context, commitID uuid.UUID) (string, error)
	GetCommitIDByName(ctx context.Context, teamID, rootCommit uuid.UUID, name string) (uuid.UUID, error)
	SetCommitName(ctx context.Context, teamID, rootCommit, commitID uuid.UUID, name string) error
}
