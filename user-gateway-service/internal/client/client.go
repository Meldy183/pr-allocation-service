package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// PRAllocationClient is a client for pr-allocation-service
type PRAllocationClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewPRAllocationClient creates a new PR allocation client
func NewPRAllocationClient(baseURL string) *PRAllocationClient {
	return &PRAllocationClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TeamMember represents a team member
type TeamMember struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

// Team represents a team
type Team struct {
	TeamName string       `json:"team_name"`
	Members  []TeamMember `json:"members"`
}

// User represents a user
type User struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	TeamID   uuid.UUID `json:"team_id"`
	TeamName string    `json:"team_name"`
	IsActive bool      `json:"is_active"`
}

// PRResponse represents PR response from pr-allocation-service
type PRResponse struct {
	PRID              string     `json:"pull_request_id"`
	PRName            string     `json:"pull_request_name"`
	AuthorID          string     `json:"author_id"`
	Status            string     `json:"status"`
	AssignedReviewers []string   `json:"assigned_reviewers"`
	CreatedAt         time.Time  `json:"createdAt"`
	MergedAt          *time.Time `json:"mergedAt"`
}

// TeamResponse represents team response with UUID
type TeamResponse struct {
	TeamID   uuid.UUID    `json:"team_id"`
	TeamName string       `json:"team_name"`
	Members  []TeamMember `json:"members"`
}

// CreateTeam creates a new team
func (c *PRAllocationClient) CreateTeam(ctx context.Context, teamName string, members []TeamMember) (*TeamResponse, error) {
	url := fmt.Sprintf("%s/team/add", c.baseURL)

	body := map[string]any{
		"team_name": teamName,
		"members":   members,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		return nil, fmt.Errorf("team already exists")
	}
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Team TeamResponse `json:"team"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.Team, nil
}

// GetTeam gets a team by name
func (c *PRAllocationClient) GetTeam(ctx context.Context, teamName string) (*TeamResponse, error) {
	url := fmt.Sprintf("%s/team/get?team_name=%s", c.baseURL, teamName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("team not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var team TeamResponse
	if err := json.NewDecoder(resp.Body).Decode(&team); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &team, nil
}

// ResolveTeamID resolves team name to team UUID
func (c *PRAllocationClient) ResolveTeamID(ctx context.Context, teamName string) (uuid.UUID, error) {
	url := fmt.Sprintf("%s/team/resolve?team_name=%s", c.baseURL, teamName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return uuid.Nil, fmt.Errorf("team not found")
	}
	if resp.StatusCode != http.StatusOK {
		return uuid.Nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		TeamName string    `json:"team_name"`
		TeamID   uuid.UUID `json:"team_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return uuid.Nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.TeamID, nil
}

// GetUser gets user by ID
func (c *PRAllocationClient) GetUser(ctx context.Context, userID string) (*User, error) {
	url := fmt.Sprintf("%s/users/get?user_id=%s", c.baseURL, userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("user not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		User User `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.User, nil
}

// GetUserByID finds user in team by user_id (deprecated, use GetUser)
func (c *PRAllocationClient) GetUserByID(ctx context.Context, userID string) (*User, string, error) {
	// We need to find the user's team first
	// This is a simplified approach - in real world we'd have a dedicated endpoint
	// For now, we'll need to iterate through teams or have the team info cached
	// Let's assume we have an endpoint or we search by getting PRs first

	// Get PRs by this user to find their team
	url := fmt.Sprintf("%s/users/getReview?user_id=%s", c.baseURL, userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// If 404, user not found
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", nil
	}

	// For now, return minimal user info from response
	// The actual implementation would need a proper user lookup endpoint
	return nil, "", nil
}

// CreatePR creates a new pull request
func (c *PRAllocationClient) CreatePR(ctx context.Context, prID, prName, authorID string) (*PRResponse, error) {
	url := fmt.Sprintf("%s/pullRequest/create", c.baseURL)

	body := map[string]string{
		"pull_request_id":   prID,
		"pull_request_name": prName,
		"author_id":         authorID,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil, fmt.Errorf("PR already exists")
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		PR PRResponse `json:"pr"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.PR, nil
}

// MergePR marks PR as merged
func (c *PRAllocationClient) MergePR(ctx context.Context, prID string) (*PRResponse, error) {
	url := fmt.Sprintf("%s/pullRequest/merge", c.baseURL)

	body := map[string]string{
		"pull_request_id": prID,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		PR PRResponse `json:"pr"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.PR, nil
}

// ApprovePR approves a PR by a reviewer
func (c *PRAllocationClient) ApprovePR(ctx context.Context, prID, reviewerID string) (*PRResponse, bool, error) {
	url := fmt.Sprintf("%s/pullRequest/approve", c.baseURL)

	body := map[string]string{
		"pull_request_id": prID,
		"reviewer_id":     reviewerID,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, false, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, false, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, false, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		PR          PRResponse `json:"pr"`
		AllApproved bool       `json:"all_approved"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, false, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.PR, result.AllApproved, nil
}

// RejectPR rejects a PR by a reviewer
func (c *PRAllocationClient) RejectPR(ctx context.Context, prID, reviewerID, reason string) (*PRResponse, error) {
	url := fmt.Sprintf("%s/pullRequest/reject", c.baseURL)

	body := map[string]string{
		"pull_request_id": prID,
		"reviewer_id":     reviewerID,
		"reason":          reason,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		PR PRResponse `json:"pr"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.PR, nil
}

// GetPRsByAuthor gets PRs by author
func (c *PRAllocationClient) GetPRsByAuthor(ctx context.Context, authorID string) ([]PRResponse, error) {
	url := fmt.Sprintf("%s/users/getAuthored?user_id=%s", c.baseURL, authorID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []PRResponse{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		PRs []PRResponse `json:"pull_requests"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.PRs, nil
}

// GetPRsByReviewer gets PRs where user is reviewer
func (c *PRAllocationClient) GetPRsByReviewer(ctx context.Context, reviewerID string) ([]PRResponse, error) {
	url := fmt.Sprintf("%s/users/getReview?user_id=%s", c.baseURL, reviewerID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []PRResponse{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		PRs []PRResponse `json:"pull_requests"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.PRs, nil
}

// CodeStorageClient is a client for code-storage-service
type CodeStorageClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewCodeStorageClient creates a new code storage client
func NewCodeStorageClient(baseURL string) *CodeStorageClient {
	return &CodeStorageClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // Longer timeout for file uploads
		},
	}
}

// CommitResponse represents commit response from code-storage-service
type CommitResponse struct {
	CommitID        uuid.UUID   `json:"commit_id"`
	TeamID          uuid.UUID   `json:"team_id"`
	RootCommit      uuid.UUID   `json:"root_commit"`
	ParentCommitIDs []uuid.UUID `json:"parent_commit_ids"`
	CreatedAt       time.Time   `json:"createdAt"`
	CommitName      *string     `json:"commit_name,omitempty"`
}

// InitRepository initializes a new repository
func (c *CodeStorageClient) InitRepository(ctx context.Context, teamID uuid.UUID, code []byte) (*CommitResponse, error) {
	url := fmt.Sprintf("%s/storage/init", c.baseURL)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("team_id", teamID.String())

	part, err := writer.CreateFormFile("code", "code.zip")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(code); err != nil {
		return nil, fmt.Errorf("failed to write code: %w", err)
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Commit CommitResponse `json:"commit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.Commit, nil
}

// Push creates a new commit
func (c *CodeStorageClient) Push(ctx context.Context, teamID, rootCommit, parentCommit uuid.UUID, code []byte) (*CommitResponse, error) {
	url := fmt.Sprintf("%s/storage/push", c.baseURL)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("team_id", teamID.String())
	_ = writer.WriteField("root_commit", rootCommit.String())
	_ = writer.WriteField("commit_id", parentCommit.String())

	part, err := writer.CreateFormFile("code", "code.zip")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(code); err != nil {
		return nil, fmt.Errorf("failed to write code: %w", err)
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("commit not found")
	}
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Commit CommitResponse `json:"commit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.Commit, nil
}

// Checkout retrieves code for a commit
func (c *CodeStorageClient) Checkout(ctx context.Context, teamID, rootCommit, commitID uuid.UUID) ([]byte, error) {
	url := fmt.Sprintf("%s/storage/checkout?team_id=%s&root_commit=%s&commit_id=%s",
		c.baseURL, teamID.String(), rootCommit.String(), commitID.String())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("commit not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// Merge merges two commits
func (c *CodeStorageClient) Merge(ctx context.Context, teamID, rootCommit, commitID1, commitID2 uuid.UUID) (*CommitResponse, error) {
	url := fmt.Sprintf("%s/storage/merge", c.baseURL)

	body := map[string]string{
		"team_id":     teamID.String(),
		"root_commit": rootCommit.String(),
		"commit_id1":  commitID1.String(),
		"commit_id2":  commitID2.String(),
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Commit CommitResponse `json:"commit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.Commit, nil
}

// ListCommits returns all commits for a repository
func (c *CodeStorageClient) ListCommits(ctx context.Context, teamID, rootCommit uuid.UUID) ([]CommitResponse, error) {
	url := fmt.Sprintf("%s/storage/commits?team_id=%s&root_commit=%s", c.baseURL, teamID.String(), rootCommit.String())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Commits []CommitResponse `json:"commits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Commits, nil
}

// InitRepositoryWithName initializes a new repository with a name
func (c *CodeStorageClient) InitRepositoryWithName(ctx context.Context, teamID uuid.UUID, repoName string, code []byte) (*CommitResponse, error) {
	url := fmt.Sprintf("%s/storage/init", c.baseURL)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("team_id", teamID.String())
	_ = writer.WriteField("commit_name", repoName) // repo name = root commit name

	part, err := writer.CreateFormFile("code", "code.zip")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(code); err != nil {
		return nil, fmt.Errorf("failed to write code: %w", err)
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Commit CommitResponse `json:"commit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.Commit, nil
}

// PushWithName creates a new commit with a name
func (c *CodeStorageClient) PushWithName(ctx context.Context, teamID, rootCommit, parentCommit uuid.UUID, commitName string, code []byte) (*CommitResponse, error) {
	url := fmt.Sprintf("%s/storage/push", c.baseURL)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("team_id", teamID.String())
	_ = writer.WriteField("root_commit", rootCommit.String())
	_ = writer.WriteField("commit_id", parentCommit.String())
	_ = writer.WriteField("commit_name", commitName)

	part, err := writer.CreateFormFile("code", "code.zip")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(code); err != nil {
		return nil, fmt.Errorf("failed to write code: %w", err)
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("commit not found")
	}
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Commit CommitResponse `json:"commit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.Commit, nil
}

// GetCommitIDByName gets commit ID by name from code-storage
func (c *CodeStorageClient) GetCommitIDByName(ctx context.Context, teamID, rootCommit uuid.UUID, commitName string) (uuid.UUID, error) {
	url := fmt.Sprintf("%s/storage/commitID?team_id=%s&root_commit=%s&name=%s",
		c.baseURL, teamID.String(), rootCommit.String(), commitName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return uuid.Nil, fmt.Errorf("commit not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return uuid.Nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		CommitID uuid.UUID `json:"commit_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return uuid.Nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.CommitID, nil
}

// ResolveCommitByName is deprecated, use GetCommitIDByName instead
func (c *CodeStorageClient) ResolveCommitByName(ctx context.Context, teamID uuid.UUID, repoName, commitName string) (uuid.UUID, error) {
	// This method is no longer used - commits are resolved via GetCommitIDByName
	// which requires root_commit UUID
	return uuid.Nil, fmt.Errorf("ResolveCommitByName is deprecated, use service.resolveCommitByName instead")
}
