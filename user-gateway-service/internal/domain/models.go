package domain

import (
	"time"

	"github.com/google/uuid"
)

// UserProfile represents user profile information
type UserProfile struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	TeamID   uuid.UUID `json:"team_id"`
	TeamName string    `json:"team_name"`
	IsActive bool      `json:"is_active"`
}

// Commit represents a code commit
type Commit struct {
	CommitID        uuid.UUID   `json:"commit_id"`
	RootCommit      uuid.UUID   `json:"root_commit"`
	ParentCommitIDs []uuid.UUID `json:"parent_commit_ids"`
	CommitName      *string     `json:"commit_name,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
}

// PullRequest represents a pull request
type PullRequest struct {
	PRID         string     `json:"pr_id"`
	Title        string     `json:"title"`
	AuthorID     string     `json:"author_id"`
	AuthorName   string     `json:"author_name,omitempty"`
	Status       string     `json:"status"`
	ReviewerIDs  []string   `json:"reviewer_ids"`
	SourceCommit uuid.UUID  `json:"source_commit"`
	TargetCommit uuid.UUID  `json:"target_commit"`
	RootCommit   uuid.UUID  `json:"root_commit"`
	CreatedAt    time.Time  `json:"created_at"`
	MergedAt     *time.Time `json:"merged_at,omitempty"`
}

// CreatePRRequest is the request for creating a PR
type CreatePRRequest struct {
	Title        string    `json:"title"`
	RootCommit   uuid.UUID `json:"root_commit"`
	SourceCommit uuid.UUID `json:"source_commit"`
	TargetCommit uuid.UUID `json:"target_commit"`
}

// RejectPRRequest is the request for rejecting a PR
type RejectPRRequest struct {
	Reason string `json:"reason,omitempty"`
}
