package cli

import (
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/source"
)

func TestResolveToken(t *testing.T) {
	github := source.Source{Host: "github.com", Owner: "o", Repo: "r"}
	gitlab := source.Source{Host: "gitlab.com", Owner: "g", Repo: "p"}
	env := func(vars map[string]string) func(string) string {
		return func(name string) string { return vars[name] }
	}

	cases := []struct {
		name string
		flag string
		src  source.Source
		vars map[string]string
		want string
	}{
		{"flag wins", "flag", github, map[string]string{"GH_TOKEN": "gh"}, "flag"},
		{"GH_TOKEN first", "", github, map[string]string{"GH_TOKEN": "gh", "GITHUB_TOKEN": "github"}, "gh"},
		{"GITHUB_TOKEN", "", github, map[string]string{"GITHUB_TOKEN": "github"}, "github"},
		{"nothing set", "", github, nil, ""},
		{"never leaks to other hosts", "", gitlab, map[string]string{"GITHUB_TOKEN": "github"}, ""},
		{"flag for other hosts", "glpat", gitlab, nil, "glpat"},
	}
	for _, tc := range cases {
		if got := resolveToken(tc.flag, tc.src, env(tc.vars)); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
