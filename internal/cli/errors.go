package cli

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dagimg-dot/gitsnip/internal/apperr"
)

type usageError struct {
	message string
	hint    string
}

func (e *usageError) Error() string {
	return e.message
}

func usage(message, hint string) error {
	return &usageError{message: message, hint: hint}
}

func describe(err error, branchFlag bool) (message, hint, detail string) {
	var ue *usageError
	var ae *apperr.Error
	switch {
	case errors.As(err, &ue):
		return ue.message, ue.hint, ""
	case errors.Is(err, context.Canceled):
		return "Cancelled", "", ""
	case errors.As(err, &ae):
		hint = ae.Hint
		if branchFlag && errors.Is(err, apperr.ErrRefNotFound) && strings.HasPrefix(hint, "the default branch is") {
			hint += "; drop -b to use it"
		}
		if ae.Cause != nil {
			detail = ae.Cause.Error()
		}
		return ae.Message, hint, detail
	}
	return capitalize(err.Error()), "", ""
}

func exitCode(err error) int {
	var ue *usageError
	switch {
	case errors.As(err, &ue):
		return 2
	case errors.Is(err, context.Canceled):
		return 130
	}
	return 1
}

func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}
