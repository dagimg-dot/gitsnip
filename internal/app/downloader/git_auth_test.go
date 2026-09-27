package downloader

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/app/gitutil"
)

func TestAuthEnvSendsTheTokenOnlyToTheSourceHost(t *testing.T) {
	if !gitutil.IsGitInstalled() {
		t.Skip("git is not installed")
	}
	t.Setenv("GIT_CONFIG_COUNT", "")
	env := authEnv("https://github.com/o/r", "ghp_secret")

	header := func(target string) string {
		out, _ := gitutil.RunGitCommand(context.Background(), t.TempDir(), env, "config", "--get-urlmatch", "http.extraheader", target)
		return strings.TrimSpace(out)
	}

	want := "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:ghp_secret"))
	if got := header("https://github.com/o/r"); got != want {
		t.Errorf("github.com header = %q, want %q", got, want)
	}
	if got := header("https://gitlab.com/o/r"); got != "" {
		t.Errorf("gitlab.com received %q", got)
	}
}

func TestAuthEnvKeepsExistingConfigEntries(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "2")
	env := authEnv("https://gitlab.com/group/project.git", "glpat")
	want := []string{
		"GIT_CONFIG_COUNT=3",
		"GIT_CONFIG_KEY_2=http.https://gitlab.com/.extraheader",
		"GIT_CONFIG_VALUE_2=Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte("oauth2:glpat")),
	}
	if strings.Join(env, "\n") != strings.Join(want, "\n") {
		t.Errorf("env = %q", env)
	}
}

func TestAuthEnvIgnoresNonHTTPSRemotes(t *testing.T) {
	for _, remote := range []string{"git@github.com:o/r.git", "file:///tmp/r.git", "http://example.com/r.git"} {
		if env := authEnv(remote, "token"); env != nil {
			t.Errorf("%s got %q", remote, env)
		}
	}
	if env := authEnv("https://github.com/o/r", ""); env != nil {
		t.Errorf("empty token got %q", env)
	}
}
