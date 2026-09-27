package downloader

import (
	"strings"
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/pathspec"
)

func TestSparseRulesAddTheAttributesGitReadsForEachPath(t *testing.T) {
	cases := []struct {
		paths []string
		want  string
	}{
		{[]string{"docs"}, "/.gitattributes /docs"},
		{[]string{"src/app/main.go"}, "/.gitattributes /src/.gitattributes /src/app/.gitattributes /src/app/main.go"},
		{[]string{"data/*.json"}, "/.gitattributes /data/**/.gitattributes /data/*.json"},
		{[]string{"**/*.md"}, "/**/*.md /**/.gitattributes /.gitattributes"},
		{[]string{"src/a.txt", "src/b.txt"}, "/.gitattributes /src/.gitattributes /src/a.txt /src/b.txt"},
		{[]string{"docs", "."}, ""},
		{nil, ""},
	}
	for _, tc := range cases {
		paths, err := pathspec.ParseAll(tc.paths)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(sparseRules(paths), " "); got != tc.want {
			t.Errorf("%q: rules = %q, want %q", tc.paths, got, tc.want)
		}
	}
}
