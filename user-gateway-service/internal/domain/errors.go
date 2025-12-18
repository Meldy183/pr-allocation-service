package domain

import "errors"

// Error codes
const (
	ErrCodeAccessDenied    = "ACCESS_DENIED"
	ErrCodeUserNotFound    = "USER_NOT_FOUND"
	ErrCodeUserInactive    = "USER_INACTIVE"
	ErrCodeTeamNotFound    = "TEAM_NOT_FOUND"
	ErrCodeCommitNotFound  = "COMMIT_NOT_FOUND"
	ErrCodePRNotFound      = "PR_NOT_FOUND"
	ErrCodePRAlreadyExists = "PR_ALREADY_EXISTS"
	ErrCodePRAlreadyMerged = "PR_ALREADY_MERGED"
	ErrCodeNotReviewer     = "NOT_REVIEWER"
	ErrCodeInvalidRequest  = "INVALID_REQUEST"
	ErrCodeInternalError   = "INTERNAL_ERROR"
	ErrCodeTeamExists      = "TEAM_EXISTS"
)

// Domain errors
var (
	ErrAccessDenied    = errors.New("access denied")
	ErrUserNotFound    = errors.New("user not found")
	ErrUserInactive    = errors.New("user is inactive")
	ErrTeamNotFound    = errors.New("team not found")
	ErrTeamExists      = errors.New("team already exists")
	ErrCommitNotFound  = errors.New("commit not found")
	ErrPRNotFound      = errors.New("pull request not found")
	ErrPRAlreadyExists = errors.New("pull request already exists")
	ErrPRAlreadyMerged = errors.New("pull request already merged")
	ErrNotReviewer     = errors.New("user is not a reviewer of this PR")
	ErrInvalidRequest  = errors.New("invalid request")
	ErrInternalError   = errors.New("internal error")
)

// ErrorResponse represents API error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error details
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
	case errors.Is(err, ErrAccessDenied):
		return ErrCodeAccessDenied
	case errors.Is(err, ErrUserNotFound):
		return ErrCodeUserNotFound
	case errors.Is(err, ErrUserInactive):
		return ErrCodeUserInactive
	case errors.Is(err, ErrTeamNotFound):
		return ErrCodeTeamNotFound
	case errors.Is(err, ErrTeamExists):
		return ErrCodeTeamExists
	case errors.Is(err, ErrCommitNotFound):
		return ErrCodeCommitNotFound
	case errors.Is(err, ErrPRNotFound):
		return ErrCodePRNotFound
	case errors.Is(err, ErrPRAlreadyExists):
		return ErrCodePRAlreadyExists
	case errors.Is(err, ErrPRAlreadyMerged):
		return ErrCodePRAlreadyMerged
	case errors.Is(err, ErrNotReviewer):
		return ErrCodeNotReviewer
	case errors.Is(err, ErrInvalidRequest):
		return ErrCodeInvalidRequest
	default:
		return ErrCodeInternalError
	}
}
