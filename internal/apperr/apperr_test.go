package apperr_test

import (
	"errors"
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/apperr"
)

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
