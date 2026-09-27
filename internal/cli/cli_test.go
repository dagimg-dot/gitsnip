package cli_test

import (
	"os"
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/cli"
)

func TestCLI_noArgs(t *testing.T) {
	if err := cli.Execute(); err != nil {
		t.Fatalf("unexpected error with no args: %v", err)
	}
}

func TestCLI_help(t *testing.T) {
	os.Args = []string{"gitsnip", "--help"}
	if err := cli.Execute(); err != nil {
		t.Fatalf("help should not error: %v", err)
	}
}
