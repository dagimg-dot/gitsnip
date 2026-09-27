package downloader

import (
	"context"
	"errors"
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/app/gitutil"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
)

func gitErr(args []string, stderr string) error {
	return &gitutil.Error{Args: args, Stderr: stderr, Err: errors.New("exit status 128")}
}

func TestGitFailureClassifiesStderr(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
		want   error
	}{
		{"missing fetch ref", "fatal: couldn't find remote ref main", apperr.ErrRefNotFound},
		{"missing clone branch", "fatal: Remote branch nosuch not found in upstream origin", apperr.ErrRefNotFound},
		{"missing commit", "fatal: git upload-pack: not our ref 0123456789abcdef0123456789abcdef01234567\nfatal: remote error: upload-pack: not our ref 0123456789abcdef0123456789abcdef01234567", apperr.ErrRefNotFound},
		{"https without credentials", "fatal: could not read Username for 'https://github.com': terminal prompts disabled", apperr.ErrRepositoryNotFound},
		{"github 404", "remote: Repository not found.\nfatal: repository 'https://github.com/o/r.git/' not found", apperr.ErrRepositoryNotFound},
		{"missing local repo", "fatal: '/tmp/nope.git' does not appear to be a git repository\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights\nand the repository exists.", apperr.ErrRepositoryNotFound},
		{"rejected token", "remote: Invalid username or token. Password authentication is not supported for Git operations.\nfatal: Authentication failed for 'https://github.com/o/r.git/'", apperr.ErrAuthenticationRequired},
		{"rejected ssh key", "git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository.", apperr.ErrAuthenticationRequired},
		{"unknown ssh host", "Host key verification failed.\nfatal: Could not read from remote repository.", apperr.ErrAuthenticationRequired},
		{"dns failure", "fatal: unable to access 'https://github.com/o/r.git/': Could not resolve host: github.com", apperr.ErrNetworkFailure},
		{"stalled transfer", "fatal: unable to access 'https://github.com/o/r/': Operation too slow. Less than 1000 bytes/sec transferred the last 60 seconds", apperr.ErrNetworkFailure},
		{"anything else", "error: something odd happened", apperr.ErrGitCommandFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := gitFailure(gitErr([]string{"fetch"}, tc.stderr), "o/r", "main")
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			var gerr *gitutil.Error
			if !errors.As(err, &gerr) {
				t.Fatal("classified error lost the underlying git error")
			}
		})
	}
}

func TestGitFailureMessages(t *testing.T) {
	cases := []struct {
		err  error
		repo string
		ref  string
		want string
	}{
		{gitErr([]string{"fetch"}, "fatal: couldn't find remote ref main"), "torvalds/linux", "main", `Branch or tag "main" doesn't exist in torvalds/linux`},
		{gitErr([]string{"fetch"}, "fatal: couldn't find remote ref HEAD"), "o/r", "", "Couldn't find the default branch of o/r"},
		{gitErr([]string{"checkout"}, "warning: noise\nerror: something odd happened"), "o/r", "main", "git checkout failed: something odd happened"},
		{gitErr([]string{"fetch"}, "fatal: unable to access 'https://github.com/o/r/': Could not resolve host: github.com"), "https://github.com/o/r", "main", "Couldn't reach github.com"},
		{gitErr([]string{"fetch"}, "ssh: Could not resolve hostname example.org: Name or service not known"), "git@example.org:o/r.git", "main", "Couldn't reach example.org"},
	}
	for _, tc := range cases {
		if got := gitFailure(tc.err, tc.repo, tc.ref).Error(); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

func TestGitFailurePassesThroughOtherErrors(t *testing.T) {
	if err := gitFailure(context.Canceled, "o/r", "main"); !errors.Is(err, context.Canceled) {
		t.Errorf("got %v, want context.Canceled", err)
	}
	plain := errors.New("boom")
	if err := gitFailure(plain, "o/r", "main"); err != plain {
		t.Errorf("got %v, want the original error", err)
	}
}
