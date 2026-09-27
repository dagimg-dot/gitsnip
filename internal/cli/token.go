package cli

import (
	"strings"

	"github.com/dagimg-dot/gitsnip/internal/source"
)

func resolveToken(flag string, src source.Source, getenv func(string) string) string {
	if flag != "" {
		return flag
	}
	if !src.GitHub() {
		return ""
	}
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if value := strings.TrimSpace(getenv(name)); value != "" {
			return value
		}
	}
	return ""
}
