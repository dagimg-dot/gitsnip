package pathspec

import "testing"

func mustParse(t *testing.T, raw string) Pattern {
	t.Helper()
	p, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse(%q): %v", raw, err)
	}
	return p
}

func TestParseNormalizes(t *testing.T) {
	cases := map[string]string{
		"src/components":    "src/components",
		"./src/components/": "src/components",
		"/src//components":  "src/components",
		`src\components`:    "src/components",
		".":                 "",
		"":                  "",
	}
	for raw, want := range cases {
		if got := mustParse(t, raw).String(); got != want {
			t.Errorf("Parse(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseRejectsEscapesAndBadGlobs(t *testing.T) {
	for _, raw := range []string{"../etc", "src/../../x", "src/[a-"} {
		if _, err := Parse(raw); err == nil {
			t.Errorf("Parse(%q) should fail", raw)
		}
	}
}

func TestMatch(t *testing.T) {
	cases := []struct {
		pattern string
		file    string
		want    bool
	}{
		{"src/components", "src/components/Button.tsx", true},
		{"src/components", "src/components/icons/star.svg", true},
		{"src/components", "src/components", true},
		{"src/components", "src/componentsX/a", false},
		{"src/components", "src", false},
		{"data/*_linux.json", "data/x_linux.json", true},
		{"data/*_linux.json", "data/sub/x_linux.json", false},
		{"data/*_linux.json", "data/x_mac.json", false},
		{"src/*", "src/sub/deep/file", true},
		{"**/*.md", "README.md", true},
		{"**/*.md", "docs/guide/intro.md", true},
		{"**/*.md", "docs/guide/intro.txt", false},
		{"docs/**/img", "docs/a/b/img/x.png", true},
		{"docs/**/img", "docs/img/x.png", true},
		{"", "anything/at/all", true},
	}
	for _, tc := range cases {
		if got := mustParse(t, tc.pattern).Match(tc.file); got != tc.want {
			t.Errorf("%q.Match(%q) = %v, want %v", tc.pattern, tc.file, got, tc.want)
		}
	}
}

func TestAnchor(t *testing.T) {
	cases := []struct {
		pattern string
		isFile  bool
		want    string
	}{
		{"src/components", false, "src/components"},
		{"src/components/Button.tsx", true, "src/components"},
		{"Makefile", true, ""},
		{"data/*_linux.json", false, "data"},
		{"**/*.md", false, ""},
		{"src/*/README.md", false, "src"},
	}
	for _, tc := range cases {
		if got := mustParse(t, tc.pattern).Anchor(tc.isFile); got != tc.want {
			t.Errorf("%q.Anchor(%v) = %q, want %q", tc.pattern, tc.isFile, got, tc.want)
		}
	}
}

func TestCommonDir(t *testing.T) {
	cases := []struct {
		dirs []string
		want string
	}{
		{[]string{"src/components"}, "src/components"},
		{[]string{"data", "data"}, "data"},
		{[]string{"src/a", "src/b"}, "src"},
		{[]string{"src/a", "docs"}, ""},
		{[]string{"src", ""}, ""},
		{nil, ""},
	}
	for _, tc := range cases {
		if got := CommonDir(tc.dirs); got != tc.want {
			t.Errorf("CommonDir(%q) = %q, want %q", tc.dirs, got, tc.want)
		}
	}
}

func TestRule(t *testing.T) {
	if got := mustParse(t, "./data/*_linux.json").Rule(); got != "/data/*_linux.json" {
		t.Errorf("Rule() = %q", got)
	}
}
