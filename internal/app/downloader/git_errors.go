package downloader

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/dagimg-dot/gitsnip/internal/app/gitutil"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
)

func gitFailure(err error, repo, ref string) error {
	var gerr *gitutil.Error
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &gerr) {
		return err
	}

	stderr := strings.ToLower(gerr.Stderr)
	mentions := func(needles ...string) bool {
		for _, needle := range needles {
			if strings.Contains(stderr, needle) {
				return true
			}
		}
		return false
	}

	switch {
	case mentions("couldn't find remote ref", "not found in upstream", "not our ref", "unadvertised object"):
		msg := fmt.Sprintf("Branch or tag %q doesn't exist in %s", ref, repo)
		if ref == "" {
			msg = fmt.Sprintf("Couldn't find the default branch of %s", repo)
		}
		return apperr.Wrap(apperr.ErrRefNotFound, err, msg, "")
	case mentions("could not read username", "repository not found", "does not appear to be a git repository"),
		mentions("repository '") && mentions("' not found"):
		return apperr.Wrap(apperr.ErrRepositoryNotFound, err,
			fmt.Sprintf("Repository %s doesn't exist or is private", repo),
			"check the URL; for private repositories pass a token or use an SSH URL")
	case mentions("host key verification failed"):
		return apperr.Wrap(apperr.ErrAuthenticationRequired, err,
			"SSH host key verification failed",
			"connect once with ssh to trust the host, then retry")
	case mentions("authentication failed", "invalid username or", "permission denied", "access denied", "returned error: 403"):
		return apperr.Wrap(apperr.ErrAuthenticationRequired, err,
			fmt.Sprintf("Access to %s was denied", repo),
			"check that your token or SSH key can read this repository")
	case mentions("could not resolve host", "failed to connect", "connection timed out", "connection refused", "network is unreachable", "operation timed out", "operation too slow"):
		return apperr.Wrap(apperr.ErrNetworkFailure, err,
			fmt.Sprintf("Couldn't reach %s", hostOf(repo)),
			"check your connection and try again")
	}

	command := "command"
	if len(gerr.Args) > 0 {
		command = gerr.Args[0]
	}
	return apperr.Wrap(apperr.ErrGitCommandFailed, err, fmt.Sprintf("git %s failed: %s", command, gitMessage(gerr.Stderr)), "")
}

func gitMessage(stderr string) string {
	lines := strings.Split(stderr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		for _, prefix := range []string{"fatal: ", "error: "} {
			if strings.HasPrefix(line, prefix) {
				return strings.TrimPrefix(line, prefix)
			}
		}
	}
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return "no output"
}

func hostOf(repo string) string {
	if u, err := url.Parse(repo); err == nil && u.Host != "" {
		return u.Hostname()
	}
	if at := strings.Index(repo, "@"); at >= 0 {
		rest := repo[at+1:]
		if colon := strings.Index(rest, ":"); colon > 0 {
			return rest[:colon]
		}
	}
	if slash := strings.Index(repo, "/"); slash > 0 {
		return repo[:slash]
	}
	return repo
}
