package tests

import (
	"errors"
	"testing"

	apperrors "github.com/dagimg-dot/gitsnip/internal/errors"
)

func TestParseGitHubAPIError_401(t *testing.T) {
	err := apperrors.ParseGitHubAPIError(401, `{"message":"Bad credentials"}`)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatal("expected AppError")
	}
	if !errors.Is(appErr.Err, apperrors.ErrAuthenticationRequired) {
		t.Errorf("expected ErrAuthenticationRequired, got %v", appErr.Err)
	}
	if appErr.StatusCode != 401 {
		t.Errorf("expected status 401, got %d", appErr.StatusCode)
	}
}

func TestParseGitHubAPIError_403_rateLimit(t *testing.T) {
	err := apperrors.ParseGitHubAPIError(403, `{"message":"API rate limit exceeded"}`)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatal("expected AppError")
	}
	if !errors.Is(appErr.Err, apperrors.ErrRateLimitExceeded) {
		t.Errorf("expected ErrRateLimitExceeded, got %v", appErr.Err)
	}
}

func TestParseGitHubAPIError_403_forbidden(t *testing.T) {
	err := apperrors.ParseGitHubAPIError(403, `{"message":"Forbidden"}`)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatal("expected AppError")
	}
	if !errors.Is(appErr.Err, apperrors.ErrAuthenticationRequired) {
		t.Errorf("expected ErrAuthenticationRequired, got %v", appErr.Err)
	}
}

func TestParseGitHubAPIError_404_notFound(t *testing.T) {
	err := apperrors.ParseGitHubAPIError(404, `{"message":"Not Found"}`)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatal("expected AppError")
	}
	if !errors.Is(appErr.Err, apperrors.ErrRepositoryNotFound) {
		t.Errorf("expected ErrRepositoryNotFound, got %v", appErr.Err)
	}
}

func TestParseGitHubAPIError_404_pathNotFound(t *testing.T) {
	err := apperrors.ParseGitHubAPIError(404, `{}`)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatal("expected AppError")
	}
	if !errors.Is(appErr.Err, apperrors.ErrPathNotFound) {
		t.Errorf("expected ErrPathNotFound, got %v", appErr.Err)
	}
}

func TestParseGitError_repositoryNotFound(t *testing.T) {
	err := apperrors.ParseGitError(errors.New("git error"), "repository not found")
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatal("expected AppError")
	}
	if !errors.Is(appErr.Err, apperrors.ErrRepositoryNotFound) {
		t.Errorf("expected ErrRepositoryNotFound, got %v", appErr.Err)
	}
}

func TestParseGitError_authenticationFailed(t *testing.T) {
	err := apperrors.ParseGitError(errors.New("git error"), "authentication failed")
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatal("expected AppError")
	}
	if !errors.Is(appErr.Err, apperrors.ErrAuthenticationRequired) {
		t.Errorf("expected ErrAuthenticationRequired, got %v", appErr.Err)
	}
}

func TestParseGitError_networkFailure(t *testing.T) {
	err := apperrors.ParseGitError(errors.New("git error"), "failed to connect")
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatal("expected AppError")
	}
	if !errors.Is(appErr.Err, apperrors.ErrNetworkFailure) {
		t.Errorf("expected ErrNetworkFailure, got %v", appErr.Err)
	}
}

func TestParseGitError_pathNotFound(t *testing.T) {
	err := apperrors.ParseGitError(errors.New("git error"), "pathspec 'foo' did not match any file")
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatal("expected AppError")
	}
	if !errors.Is(appErr.Err, apperrors.ErrPathNotFound) {
		t.Errorf("expected ErrPathNotFound, got %v", appErr.Err)
	}
}

func TestFormatError_AppError(t *testing.T) {
	appErr := &apperrors.AppError{
		Err:     apperrors.ErrInvalidURL,
		Message: "Bad URL",
		Hint:    "Use a valid URL",
	}
	got := apperrors.FormatError(appErr)
	want := "Bad URL\nHint: Use a valid URL\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatError_plainError(t *testing.T) {
	got := apperrors.FormatError(errors.New("plain error"))
	want := "plain error\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
