package source

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		raw  string
		want Source
	}{
		{"dagimg-dot/gitsnip", Source{Host: "github.com", Owner: "dagimg-dot", Repo: "gitsnip", URL: "https://github.com/dagimg-dot/gitsnip.git"}},
		{"dagimg-dot/gitsnip@v0.1.1", Source{Host: "github.com", Owner: "dagimg-dot", Repo: "gitsnip", Ref: "v0.1.1", URL: "https://github.com/dagimg-dot/gitsnip.git"}},
		{"owner/repo/src/lib", Source{Host: "github.com", Owner: "owner", Repo: "repo", Path: "src/lib", URL: "https://github.com/owner/repo.git"}},
		{"dagimg-dot/gitsnip/cmd@v0.1.1", Source{Host: "github.com", Owner: "dagimg-dot", Repo: "gitsnip", Ref: "v0.1.1", Path: "cmd", URL: "https://github.com/dagimg-dot/gitsnip.git"}},
		{"owner/repo/src@feature/x", Source{Host: "github.com", Owner: "owner", Repo: "repo", Ref: "feature/x", Path: "src", URL: "https://github.com/owner/repo.git"}},
		{"owner/repo/node_modules/@types", Source{Host: "github.com", Owner: "owner", Repo: "repo", Path: "node_modules/@types", URL: "https://github.com/owner/repo.git"}},
		{"github.com/o/r/docs@v2", Source{Host: "github.com", Owner: "o", Repo: "r", Ref: "v2", Path: "docs", URL: "https://github.com/o/r.git"}},
		{"gitlab.com/group/project@main", Source{Host: "gitlab.com", Owner: "group", Repo: "project", Ref: "main", URL: "https://gitlab.com/group/project"}},
		{"owner/repo.js", Source{Host: "github.com", Owner: "owner", Repo: "repo.js", URL: "https://github.com/owner/repo.js.git"}},
		{"https://github.com/dagimg-dot/gitsnip", Source{Host: "github.com", Owner: "dagimg-dot", Repo: "gitsnip", URL: "https://github.com/dagimg-dot/gitsnip.git"}},
		{"https://github.com/dagimg-dot/gitsnip/", Source{Host: "github.com", Owner: "dagimg-dot", Repo: "gitsnip", URL: "https://github.com/dagimg-dot/gitsnip.git"}},
		{"https://github.com/dagimg-dot/gitsnip.git", Source{Host: "github.com", Owner: "dagimg-dot", Repo: "gitsnip", URL: "https://github.com/dagimg-dot/gitsnip.git"}},
		{"https://www.github.com/o/r", Source{Host: "github.com", Owner: "o", Repo: "r", URL: "https://github.com/o/r.git"}},
		{"github.com/o/r", Source{Host: "github.com", Owner: "o", Repo: "r", URL: "https://github.com/o/r.git"}},
		{"https://github.com/o/r/tree/main/internal/app", Source{Host: "github.com", Owner: "o", Repo: "r", RefPath: "main/internal/app", URL: "https://github.com/o/r.git"}},
		{"https://github.com/o/r/tree/feature/x/src", Source{Host: "github.com", Owner: "o", Repo: "r", RefPath: "feature/x/src", URL: "https://github.com/o/r.git"}},
		{"https://github.com/o/r/blob/main/Makefile", Source{Host: "github.com", Owner: "o", Repo: "r", RefPath: "main/Makefile", URL: "https://github.com/o/r.git"}},
		{"https://github.com/o/r/tree/main", Source{Host: "github.com", Owner: "o", Repo: "r", RefPath: "main", URL: "https://github.com/o/r.git"}},
		{"https://github.com/o/r/commit/0123abc", Source{Host: "github.com", Owner: "o", Repo: "r", Ref: "0123abc", URL: "https://github.com/o/r.git"}},
		{"https://github.com/o/r/releases/tag/v1.2", Source{Host: "github.com", Owner: "o", Repo: "r", Ref: "v1.2", URL: "https://github.com/o/r.git"}},
		{"https://github.com/o/r/src/lib", Source{Host: "github.com", Owner: "o", Repo: "r", Path: "src/lib", URL: "https://github.com/o/r.git"}},
		{"https://gitlab.com/group/sub/project/-/tree/main/config", Source{Host: "gitlab.com", Owner: "group/sub", Repo: "project", RefPath: "main/config", URL: "https://gitlab.com/group/sub/project"}},
		{"https://gitlab.com/group/sub/project/-/blob/main/a.yml", Source{Host: "gitlab.com", Owner: "group/sub", Repo: "project", RefPath: "main/a.yml", URL: "https://gitlab.com/group/sub/project"}},
		{"gitlab.com/group/sub/project", Source{Host: "gitlab.com", Owner: "group/sub", Repo: "project", URL: "https://gitlab.com/group/sub/project"}},
		{"https://codeberg.org/owner/repo/src/branch/main/docs", Source{Host: "codeberg.org", Owner: "owner", Repo: "repo", RefPath: "main/docs", URL: "https://codeberg.org/owner/repo"}},
		{"https://codeberg.org/owner/repo/src/tag/v2/docs", Source{Host: "codeberg.org", Owner: "owner", Repo: "repo", RefPath: "v2/docs", URL: "https://codeberg.org/owner/repo"}},
		{"https://bitbucket.org/owner/repo/src/main/docs", Source{Host: "bitbucket.org", Owner: "owner", Repo: "repo", RefPath: "main/docs", URL: "https://bitbucket.org/owner/repo"}},
		{"git.sr.ht/~user/tools", Source{Host: "git.sr.ht", Owner: "~user", Repo: "tools", URL: "https://git.sr.ht/~user/tools"}},
		{"https://git.sr.ht/~user/tools/tree/master/item/data", Source{Host: "git.sr.ht", Owner: "~user", Repo: "tools", Ref: "master", Path: "data", URL: "https://git.sr.ht/~user/tools"}},
		{"https://git.example.com/team/repo/tree/dev/x", Source{Host: "git.example.com", Owner: "team", Repo: "repo", RefPath: "dev/x", URL: "https://git.example.com/team/repo"}},
		{"http://localhost:3000/o/r", Source{Host: "localhost", Owner: "o", Repo: "r", URL: "http://localhost:3000/o/r"}},
		{"git@github.com:owner/repo.git", Source{Host: "github.com", Owner: "owner", Repo: "repo", URL: "git@github.com:owner/repo.git"}},
		{"git@github.com:owner/repo.git@v1", Source{Host: "github.com", Owner: "owner", Repo: "repo", Ref: "v1", URL: "git@github.com:owner/repo.git"}},
		{"git@gitlab.com:group/sub/project.git", Source{Host: "gitlab.com", Owner: "group/sub", Repo: "project", URL: "git@gitlab.com:group/sub/project.git"}},
		{"ssh://git@example.com:2222/owner/repo.git", Source{Host: "example.com", Owner: "owner", Repo: "repo", URL: "ssh://git@example.com:2222/owner/repo.git"}},
		{"file:///tmp/fixtures/repo.git", Source{Repo: "repo", URL: "file:///tmp/fixtures/repo.git"}},
		{"file:///tmp/fixtures/repo.git@trunk", Source{Repo: "repo", Ref: "trunk", URL: "file:///tmp/fixtures/repo.git"}},
		{"  owner/repo  ", Source{Host: "github.com", Owner: "owner", Repo: "repo", URL: "https://github.com/owner/repo.git"}},
	}
	for _, tc := range cases {
		got, err := Parse(tc.raw)
		if err != nil {
			t.Errorf("Parse(%q): %v", tc.raw, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Parse(%q)\n got %+v\nwant %+v", tc.raw, got, tc.want)
		}
	}
}

func TestParseRejects(t *testing.T) {
	for _, raw := range []string{
		"",
		"gitsnip",
		"owner/",
		"ftp://example.com/o/r",
		"https://github.com/owner",
		"owner/re po",
		"https://example.com/a/b/c/-/../x",
		"https://github.com/o/r@v1/tree/main/x",
		"github.com/o/r/tree/main/x@v2",
	} {
		if got, err := Parse(raw); err == nil {
			t.Errorf("Parse(%q) = %+v, want an error", raw, got)
		}
	}
}

func TestDisplay(t *testing.T) {
	cases := map[string]string{
		"dagimg-dot/gitsnip":                  "dagimg-dot/gitsnip",
		"git@github.com:o/r.git":              "o/r",
		"git.sr.ht/~user/tools":               "git.sr.ht/~user/tools",
		"https://gitlab.com/group/sub/proj":   "gitlab.com/group/sub/proj",
		"file:///tmp/fixtures/snippets.git":   "snippets",
		"ssh://git@example.com:2222/o/r.git":  "example.com/o/r",
		"https://codeberg.org/owner/repo.git": "codeberg.org/owner/repo",
	}
	for raw, want := range cases {
		src, err := Parse(raw)
		if err != nil {
			t.Fatalf("Parse(%q): %v", raw, err)
		}
		if got := src.Display(); got != want {
			t.Errorf("Display(%q) = %q, want %q", raw, got, want)
		}
	}
}
