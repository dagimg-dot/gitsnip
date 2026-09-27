package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/dagimg-dot/gitsnip/internal/app"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/pathspec"
	"github.com/dagimg-dot/gitsnip/internal/source"
	"github.com/dagimg-dot/gitsnip/internal/ui"
)

type jsonResult struct {
	Repo     string   `json:"repo"`
	URL      string   `json:"url"`
	Ref      string   `json:"ref"`
	Commit   string   `json:"commit"`
	Method   string   `json:"method"`
	Paths    []string `json:"paths"`
	Output   string   `json:"output"`
	Files    int      `json:"files"`
	Bytes    int64    `json:"bytes"`
	Ms       int64    `json:"ms"`
	Warnings []string `json:"warnings,omitempty"`
}

func summarize(src source.Source, res model.Result, elapsed time.Duration) ui.Summary {
	return ui.Summary{
		Repo:    src.Display(),
		Ref:     refLabel(res.Ref, res.Commit),
		What:    pathsLabel(res.Paths),
		Target:  app.DisplayPath(res.Target),
		Files:   res.Files,
		Bytes:   res.Bytes,
		Elapsed: elapsed,
	}
}

func writeJSON(w io.Writer, src source.Source, res model.Result, warnings []string, elapsed time.Duration) error {
	output, err := filepath.Abs(res.Output)
	if err != nil {
		output = res.Output
	}
	return json.NewEncoder(w).Encode(jsonResult{
		Repo:     src.Display(),
		URL:      src.URL,
		Ref:      res.Ref,
		Commit:   res.Commit,
		Method:   string(res.Method),
		Paths:    namedPaths(res.Paths),
		Output:   output,
		Files:    res.Files,
		Bytes:    res.Bytes,
		Ms:       elapsed.Milliseconds(),
		Warnings: warnings,
	})
}

func refLabel(ref, commit string) string {
	if ref == "" || ref == commit {
		return shortCommit(commit)
	}
	return ref
}

func shortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

func pathsLabel(paths []pathspec.Pattern) string {
	named := namedPaths(paths)
	switch len(named) {
	case 0:
		return ""
	case 1:
		return named[0]
	}
	return fmt.Sprintf("%d paths", len(named))
}

func namedPaths(paths []pathspec.Pattern) []string {
	named := []string{}
	for _, p := range paths {
		if !p.IsAll() {
			named = append(named, p.String())
		}
	}
	return named
}
