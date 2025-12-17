package domain

import "github.com/google/uuid"

// Team represents a team that owns repositories
type Team struct {
	ID   uuid.UUID `json:"team_id"`
	Name string    `json:"name"`
}
