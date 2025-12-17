package domain

import (
	"time"

	"github.com/google/uuid"
)

// Commit represents a single commit in the repository tree
type Commit struct {
	ID              uuid.UUID   `json:"commit_id"`
	TeamID          uuid.UUID   `json:"team_id"`
	RootCommit      uuid.UUID   `json:"root_commit"`
	ParentCommitIDs []uuid.UUID `json:"parent_commit_ids"`
	Code            []byte      `json:"-"`
	CreatedAt       time.Time   `json:"createdAt"`
	CommitName      *string     `json:"commit_name,omitempty"`
}

// CommitResponse is the JSON response for commit operations
type CommitResponse struct {
	Commit *CommitDTO `json:"commit"`
}

// CommitDTO is the data transfer object for commit
type CommitDTO struct {
	CommitID        uuid.UUID   `json:"commit_id"`
	TeamID          uuid.UUID   `json:"team_id"`
	RootCommit      uuid.UUID   `json:"root_commit"`
	ParentCommitIDs []uuid.UUID `json:"parent_commit_ids"`
	CreatedAt       time.Time   `json:"createdAt"`
	CommitName      *string     `json:"commit_name,omitempty"`
}

// ToDTO converts Commit to CommitDTO
func (c *Commit) ToDTO() *CommitDTO {
	return &CommitDTO{
		CommitID:        c.ID,
		TeamID:          c.TeamID,
		RootCommit:      c.RootCommit,
		ParentCommitIDs: c.ParentCommitIDs,
		CreatedAt:       c.CreatedAt,
		CommitName:      c.CommitName,
	}
}

// CommitName represents mapping between commit and its human-readable name
type CommitNameMapping struct {
	TeamID     uuid.UUID `json:"team_id"`
	RootCommit uuid.UUID `json:"root_commit"`
	CommitID   uuid.UUID `json:"commit_id"`
	Name       string    `json:"name"`
}

// MergeRequest is the request body for merge operation
type MergeRequest struct {
	TeamID     uuid.UUID `json:"team_id"`
	RootCommit uuid.UUID `json:"root_commit"`
	CommitID1  uuid.UUID `json:"commit_id1"`
	CommitID2  uuid.UUID `json:"commit_id2"`
}

// CommitNameResponse is the response for commit name lookup
type CommitNameResponse struct {
	CommitID uuid.UUID `json:"commit_id"`
	Name     string    `json:"name"`
}

// CommitIDResponse is the response for commit ID lookup by name
type CommitIDResponse struct {
	CommitID uuid.UUID `json:"commit_id"`
}
