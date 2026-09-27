package cli

import (
	"io"
	"runtime/debug"
	"strings"

	"github.com/dagimg-dot/gitsnip/internal/ui"
	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
	builtBy   = "unknown"
)

func currentVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func newVersionCmd(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:    "version",
		Short:  "Print the version",
		Hidden: true,
		Args:   cobra.NoArgs,
		Run: func(*cobra.Command, []string) {
			ui.Print(stdout, func(p ui.Paint) string {
				var details []string
				if commit != "none" {
					details = append(details, "commit "+commit)
				}
				if buildDate != "unknown" {
					details = append(details, "built "+buildDate)
				}
				if builtBy != "unknown" {
					details = append(details, "by "+builtBy)
				}
				out := p(ui.Bold, "gitsnip") + " " + currentVersion() + "\n"
				if len(details) > 0 {
					out += p(ui.Dim, strings.Join(details, " · ")) + "\n"
				}
				return out
			})
		},
	}
}
