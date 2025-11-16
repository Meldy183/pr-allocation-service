package domain
import "time"
// User represents a team member
type User struct {
UserID    string    `json:"user_id"`
Username  string    `json:"username"`
TeamName  string    `json:"team_name,omitempty"`
IsActive  bool      `json:"is_active"`
CreatedAt time.Time `json:"-"`
UpdatedAt time.Time `json:"-"`
}
// TeamMember for API response
type TeamMember struct {
UserID   string `json:"user_id"`
Username string `json:"username"`
IsActive bool   `json:"is_active"`
}
// Team represents a group of users
type Team struct {
TeamName  string       `json:"team_name"`
Members   []TeamMember `json:"members"`
CreatedAt time.Time    `json:"-"`
UpdatedAt time.Time    `json:"-"`
}
// PRStatus represents PR status
type PRStatus string
const (
StatusOpen   PRStatus = "OPEN"
StatusMerged PRStatus = "MERGED"
)
// PullRequest represents a PR
type PullRequest struct {
PullRequestID     string     `json:"pull_request_id"`
PullRequestName   string     `json:"pull_request_name"`
AuthorID          string     `json:"author_id"`
Status            PRStatus   `json:"status"`
AssignedReviewers []string   `json:"assigned_reviewers"`
CreatedAt         *time.Time `json:"createdAt,omitempty"`
MergedAt          *time.Time `json:"mergedAt,omitempty"`
}
// PullRequestShort for list responses
type PullRequestShort struct {
PullRequestID   string   `json:"pull_request_id"`
PullRequestName string   `json:"pull_request_name"`
AuthorID        string   `json:"author_id"`
Status          PRStatus `json:"status"`
}
// CreateTeamRequest - POST /team/add
type CreateTeamRequest struct {
TeamName string       `json:"team_name"`
Members  []TeamMember `json:"members"`
}
// SetUserActiveRequest - POST /users/setIsActive
type SetUserActiveRequest struct {
UserID   string `json:"user_id"`
IsActive bool   `json:"is_active"`
}
// CreatePRRequest - POST /pullRequest/create
type CreatePRRequest struct {
PullRequestID   string `json:"pull_request_id"`
PullRequestName string `json:"pull_request_name"`
AuthorID        string `json:"author_id"`
}
// MergePRRequest - POST /pullRequest/merge
type MergePRRequest struct {
PullRequestID string `json:"pull_request_id"`
}
// ReassignRequest - POST /pullRequest/reassign
type ReassignRequest struct {
PullRequestID string `json:"pull_request_id"`
OldUserID     string `json:"old_user_id"`
}
// ErrorResponse for API errors
type ErrorResponse struct {
Error ErrorDetail `json:"error"`
}
type ErrorDetail struct {
Code    string `json:"code"`
Message string `json:"message"`
}
// Error codes
const (
ErrTeamExists  = "TEAM_EXISTS"
ErrPRExists    = "PR_EXISTS"
ErrPRMerged    = "PR_MERGED"
ErrNotAssigned = "NOT_ASSIGNED"
ErrNoCandidate = "NO_CANDIDATE"
ErrNotFound    = "NOT_FOUND"
)
