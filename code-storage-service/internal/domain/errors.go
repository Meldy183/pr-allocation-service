package domain

import "errors"

// Error codes matching OpenAPI spec
const (
	ErrCodeTeamNotFound          = "TEAM_NOT_FOUND"
	ErrCodeRootCommitNotFound    = "ROOT_COMMIT_NOT_FOUND"
	ErrCodeCommitNotFound        = "COMMIT_NOT_FOUND"
	ErrCodeRepositoryAlreadyInit = "REPOSITORY_ALREADY_INITIALIZED"
	ErrCodeInvalidParent         = "INVALID_PARENT"
	ErrCodeCommitNotLeaf         = "COMMIT_NOT_LEAF"
	ErrCodeMergeConflict         = "MERGE_CONFLICT"
	ErrCodeInvalidCommitName     = "INVALID_COMMIT_NAME"
	ErrCodeCommitNameExists      = "COMMIT_NAME_EXISTS"
)

// Domain errors
var (
	ErrTeamNotFound          = errors.New("team not found")
	ErrRootCommitNotFound    = errors.New("root commit not found")
	ErrCommitNotFound        = errors.New("commit not found")
	ErrRepositoryAlreadyInit = errors.New("repository already initialized")
	ErrInvalidParent         = errors.New("invalid parent commit")
	ErrCommitNotLeaf         = errors.New("only leaf commits can be merged")
	ErrMergeConflict         = errors.New("merge conflict detected")
	ErrInvalidCommitName     = errors.New("invalid commit name")
	ErrCommitNameExists      = errors.New("commit name already exists")
)

// ErrorResponse represents API error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error code and message
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewErrorResponse creates a new error response
func NewErrorResponse(code, message string) ErrorResponse {
	return ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
}

// MapErrorToCode maps domain error to API error code
func MapErrorToCode(err error) string {
	switch {
	case errors.Is(err, ErrTeamNotFound):
		return ErrCodeTeamNotFound
	case errors.Is(err, ErrRootCommitNotFound):
		return ErrCodeRootCommitNotFound
	case errors.Is(err, ErrCommitNotFound):
		return ErrCodeCommitNotFound
	case errors.Is(err, ErrRepositoryAlreadyInit):
		return ErrCodeRepositoryAlreadyInit
	case errors.Is(err, ErrInvalidParent):
		return ErrCodeInvalidParent
	case errors.Is(err, ErrCommitNotLeaf):
		return ErrCodeCommitNotLeaf
	case errors.Is(err, ErrMergeConflict):
		return ErrCodeMergeConflict
	case errors.Is(err, ErrInvalidCommitName):
		return ErrCodeInvalidCommitName
	case errors.Is(err, ErrCommitNameExists):
		return ErrCodeCommitNameExists
	default:
		return "INTERNAL_ERROR"
	}
}
