package apperr_test

import (
	"errors"
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/apperr"
)

func TestWrapKeepsTheKindAndTheCause(t *testing.T) {
	cause := errors.New("exit status 128")
	err := apperr.Wrap(apperr.ErrRefNotFound, cause, `Branch or tag "main" doesn't exist in o/r`, `the default branch is "master"`)

	if err.Error() != `Branch or tag "main" doesn't exist in o/r` {
		t.Errorf("message = %q", err.Error())
	}
	if !errors.Is(err, apperr.ErrRefNotFound) || !errors.Is(err, cause) {
		t.Error("errors.Is should see both the kind and the cause")
	}
	if errors.Is(err, apperr.ErrPathNotFound) {
		t.Error("matched an unrelated kind")
	}
}

func TestWrapWithoutACause(t *testing.T) {
	err := apperr.Wrap(apperr.ErrUnsupported, nil, "nope", "")
	if !errors.Is(err, apperr.ErrUnsupported) {
		t.Error("lost the kind")
	}
}
