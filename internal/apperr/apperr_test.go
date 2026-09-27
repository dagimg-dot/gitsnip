package apperr_test

import (
	"errors"
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/apperr"
)

func TestParseGitHubAPIError_401(t *testing.T) {
	err := apperr.ParseGitHubAPIError(401, `{"message":"Bad credentials"}`)
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatal("expected *apperr.Error")
	}
	if !errors.Is(appErr.Err, apperr.ErrAuthenticationRequired) {
		t.Errorf("expected ErrAuthenticationRequired, got %v", appErr.Err)
	}
	if appErr.StatusCode != 401 {
		t.Errorf("expected status 401, got %d", appErr.StatusCode)
	}
}

func TestParseGitHubAPIError_403_rateLimit(t *testing.T) {
	err := apperr.ParseGitHubAPIError(403, `{"message":"API rate limit exceeded"}`)
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatal("expected *apperr.Error")
	}
	if !errors.Is(appErr.Err, apperr.ErrRateLimitExceeded) {
		t.Errorf("expected ErrRateLimitExceeded, got %v", appErr.Err)
	}
}

func TestParseGitHubAPIError_403_forbidden(t *testing.T) {
	err := apperr.ParseGitHubAPIError(403, `{"message":"Forbidden"}`)
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatal("expected *apperr.Error")
	}
	if !errors.Is(appErr.Err, apperr.ErrAuthenticationRequired) {
		t.Errorf("expected ErrAuthenticationRequired, got %v", appErr.Err)
	}
}

func TestParseGitHubAPIError_404_notFound(t *testing.T) {
	err := apperr.ParseGitHubAPIError(404, `{"message":"Not Found"}`)
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatal("expected *apperr.Error")
	}
	if !errors.Is(appErr.Err, apperr.ErrRepositoryNotFound) {
		t.Errorf("expected ErrRepositoryNotFound, got %v", appErr.Err)
	}
}

func TestParseGitHubAPIError_404_pathNotFound(t *testing.T) {
	err := apperr.ParseGitHubAPIError(404, `{}`)
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatal("expected *apperr.Error")
	}
	if !errors.Is(appErr.Err, apperr.ErrPathNotFound) {
		t.Errorf("expected ErrPathNotFound, got %v", appErr.Err)
	}
}

func TestParseGitError_repositoryNotFound(t *testing.T) {
	err := apperr.ParseGitError(errors.New("git error"), "repository not found")
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatal("expected *apperr.Error")
	}
	if !errors.Is(appErr.Err, apperr.ErrRepositoryNotFound) {
		t.Errorf("expected ErrRepositoryNotFound, got %v", appErr.Err)
	}
}

func TestParseGitError_authenticationFailed(t *testing.T) {
	err := apperr.ParseGitError(errors.New("git error"), "authentication failed")
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatal("expected *apperr.Error")
	}
	if !errors.Is(appErr.Err, apperr.ErrAuthenticationRequired) {
		t.Errorf("expected ErrAuthenticationRequired, got %v", appErr.Err)
	}
}

func TestParseGitError_networkFailure(t *testing.T) {
	err := apperr.ParseGitError(errors.New("git error"), "failed to connect")
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatal("expected *apperr.Error")
	}
	if !errors.Is(appErr.Err, apperr.ErrNetworkFailure) {
		t.Errorf("expected ErrNetworkFailure, got %v", appErr.Err)
	}
}

func TestParseGitError_pathNotFound(t *testing.T) {
	err := apperr.ParseGitError(errors.New("git error"), "pathspec 'foo' did not match any file")
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatal("expected *apperr.Error")
	}
	if !errors.Is(appErr.Err, apperr.ErrPathNotFound) {
		t.Errorf("expected ErrPathNotFound, got %v", appErr.Err)
	}
}

func TestFormatError_appError(t *testing.T) {
	appErr := &apperr.Error{
		Err:     apperr.ErrInvalidURL,
		Message: "Bad URL",
		Hint:    "Use a valid URL",
	}
	got := apperr.FormatError(appErr)
	want := "Bad URL\nHint: Use a valid URL\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatError_plainError(t *testing.T) {
	got := apperr.FormatError(errors.New("plain error"))
	want := "plain error\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
