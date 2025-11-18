package storage

import (
	"context"

	"github.com/meld/pr-allocation-service/internal/domain"
)

// Storage defines the interface for data persistence.
type Storage interface {
	// User operations
	CreateUser(ctx context.Context, user *domain.User) error
	GetUser(ctx context.Context, userID string) (*domain.User, error)
	UpdateUser(ctx context.Context, user *domain.User) error
	GetUsersByTeam(ctx context.Context, teamName string) ([]*domain.User, error)
	// Team operations
	CreateTeam(ctx context.Context, team *domain.Team) error
	GetTeam(ctx context.Context, teamName string) (*domain.Team, error)
	TeamExists(ctx context.Context, teamName string) (bool, error)
	// PR operations
	CreatePR(ctx context.Context, pr *domain.PullRequest) error
	GetPR(ctx context.Context, prID string) (*domain.PullRequest, error)
	UpdatePR(ctx context.Context, pr *domain.PullRequest) error
	GetPRsByReviewer(ctx context.Context, userID string) ([]*domain.PullRequest, error)
	PRExists(ctx context.Context, prID string) (bool, error)
}
