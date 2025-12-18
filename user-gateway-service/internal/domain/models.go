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
	RepoName        *string     `json:"repo_name,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
}

// PullRequest represents a pull request
type PullRequest struct {
	PRID             string     `json:"pr_id"`
	PRName           string     `json:"pr_name"`
	Title            string     `json:"title"`
	AuthorID         string     `json:"author_id,omitempty"`
	AuthorName       string     `json:"author_name"`
	Status           string     `json:"status"`
	ReviewerIDs      []string   `json:"reviewer_ids,omitempty"`
	ReviewerNames    []string   `json:"reviewer_names,omitempty"`
	SourceCommitID   uuid.UUID  `json:"source_commit_id,omitempty"`
	SourceCommitName string     `json:"source_commit_name,omitempty"`
	TargetCommitID   uuid.UUID  `json:"target_commit_id,omitempty"`
	TargetCommitName string     `json:"target_commit_name,omitempty"`
	RootCommitID     uuid.UUID  `json:"root_commit_id,omitempty"`
	RepoName         string     `json:"repo_name,omitempty"`
	TeamName         string     `json:"team_name,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	MergedAt         *time.Time `json:"merged_at,omitempty"`
}

// CreatePRRequest is the request for creating a PR (using names only)
type CreatePRRequest struct {
	Title            string `json:"title"`
	PRName           string `json:"pr_name"`
	TeamName         string `json:"team_name"`
	RepoName         string `json:"repo_name"`
	SourceCommitName string `json:"source_commit_name"`
	TargetCommitName string `json:"target_commit_name"`
}

// RejectPRRequest is the request for rejecting a PR
type RejectPRRequest struct {
	Reason string `json:"reason,omitempty"`
}

// CreateTeamMember is a member in CreateTeamRequest (using username only)
type CreateTeamMember struct {
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

// TeamMemberResponse is a member in response (includes ID)
type TeamMemberResponse struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

// CreateTeamRequest is the request for creating a team
type CreateTeamRequest struct {
	TeamName string             `json:"team_name"`
	Members  []CreateTeamMember `json:"members"`
}

// Team represents a team (response includes UUID)
type Team struct {
	TeamID   uuid.UUID            `json:"team_id"`
	TeamName string               `json:"team_name"`
	Members  []TeamMemberResponse `json:"members"`
}
