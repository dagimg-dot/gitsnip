package errors

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrRateLimitExceeded      = errors.New("GitHub API rate limit exceeded")
	ErrAuthenticationRequired = errors.New("authentication required for this repository")
	ErrRepositoryNotFound     = errors.New("repository not found")
	ErrPathNotFound           = errors.New("path not found in repository")
	ErrNetworkFailure         = errors.New("network connection error")
	ErrInvalidURL             = errors.New("invalid repository URL")
)

type AppError struct {
	Err        error
	Message    string
	Hint       string
	StatusCode int
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func FormatError(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("%s\n", appErr.Message))

		if appErr.Hint != "" {
			builder.WriteString(fmt.Sprintf("Hint: %s\n", appErr.Hint))
		}

		return builder.String()
	}

	return fmt.Sprintf("%v\n", err)
}

func ParseGitHubAPIError(statusCode int, body string) error {
	loweredBody := strings.ToLower(body)

	var appErr AppError
	appErr.StatusCode = statusCode

	switch statusCode {
	case 401:
		appErr.Err = ErrAuthenticationRequired
		appErr.Message = "Authentication required to access this repository"
		appErr.Hint = "Use --token flag to provide a GitHub token with appropriate permissions"

	case 403:
		if strings.Contains(loweredBody, "rate limit exceeded") {
			appErr.Err = ErrRateLimitExceeded
			appErr.Message = "GitHub API rate limit exceeded"
			appErr.Hint = "Use --token flag to provide a GitHub token to increase rate limits"
		} else {
			appErr.Err = ErrAuthenticationRequired
			appErr.Message = "Access forbidden to this repository or resource"
			appErr.Hint = "Check that your token has the correct permissions"
		}

	case 404:
		if strings.Contains(loweredBody, "not found") {
			appErr.Err = ErrRepositoryNotFound
			appErr.Message = "Repository or path not found"
			appErr.Hint = "Check that the repository URL and path are correct"
		} else {
			appErr.Err = ErrPathNotFound
			appErr.Message = "Path not found in repository"
			appErr.Hint = "Check that the folder path exists in the specified branch"
		}

	default:
		appErr.Err = errors.New(body)
		appErr.Message = fmt.Sprintf("GitHub API error (%d): %s", statusCode, body)
	}

	return &appErr
}
