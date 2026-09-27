package apperr

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrRateLimitExceeded      = errors.New("GitHub API rate limit exceeded")
	ErrAuthenticationRequired = errors.New("authentication required for this repository")
	ErrRepositoryNotFound     = errors.New("repository not found")
	ErrRefNotFound            = errors.New("ref not found")
	ErrPathNotFound           = errors.New("path not found in repository")
	ErrNetworkFailure         = errors.New("network connection error")
	ErrInvalidURL             = errors.New("invalid repository URL")
	ErrGitNotInstalled        = errors.New("git is not installed")
	ErrGitCommandFailed       = errors.New("git command failed")
	ErrUnsupported            = errors.New("unsupported")
	ErrDestinationExists      = errors.New("destination already exists")
)

type Error struct {
	Err     error
	Message string
	Hint    string
	Cause   error
}

func Wrap(kind, cause error, message, hint string) *Error {
	return &Error{Err: kind, Message: message, Hint: hint, Cause: cause}
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() []error {
	var errs []error
	if e.Err != nil {
		errs = append(errs, e.Err)
	}
	if e.Cause != nil {
		errs = append(errs, e.Cause)
	}
	return errs
}

func FormatError(err error) string {
	var appErr *Error
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
