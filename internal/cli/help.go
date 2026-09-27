package cli

import (
	"io"
	"strings"
	"unicode/utf8"

	"github.com/dagimg-dot/gitsnip/internal/ui"
	"github.com/spf13/cobra"
)

const valueAnnotation = "gitsnip/value"

var examples = []string{
	"gitsnip dagimg-dot/gitsnip internal/app",
	"gitsnip https://github.com/owner/repo/tree/main/docs",
	"gitsnip owner/repo@v1.2.0 src/lib -o vendor/lib",
	"gitsnip gitlab.com/group/project 'config/*.yml'",
}

var helpFlags = []string{"output", "branch", "method", "token", "force", "quiet", "verbose", "json", "help", "version"}

func writeHelp(cmd *cobra.Command, w io.Writer) {
	rows := flagRows(cmd)
	width := 0
	for _, row := range rows {
		width = max(width, utf8.RuneCountInString(row[0]))
	}

	ui.Print(w, func(p ui.Paint) string {
		var b strings.Builder
		b.WriteString("Download folders and files from any git repository.\n\n")
		b.WriteString(p(ui.Bold, "Usage") + "\n")
		b.WriteString("  gitsnip <source> [path...] [flags]\n\n")
		b.WriteString("  " + p(ui.Dim, "<source> is owner/repo[@ref], a link to a repository, folder or file,") + "\n")
		b.WriteString("  " + p(ui.Dim, "or any git URL. Paths can be folders, files or quoted globs.") + "\n\n")
		b.WriteString(p(ui.Bold, "Examples") + "\n")
		for _, example := range examples {
			b.WriteString("  " + p(ui.Dim, "$") + " " + example + "\n")
		}
		b.WriteString("\n" + p(ui.Bold, "Flags") + "\n")
		for _, row := range rows {
			gap := strings.Repeat(" ", width-utf8.RuneCountInString(row[0])+3)
			b.WriteString("  " + row[0] + gap + p(ui.Dim, row[1]) + "\n")
		}
		return b.String()
	})
}

func flagRows(cmd *cobra.Command) [][2]string {
	var rows [][2]string
	for _, name := range helpFlags {
		f := cmd.Flags().Lookup(name)
		if f == nil || f.Hidden {
			continue
		}
		left := "    --" + f.Name
		if f.Shorthand != "" {
			left = "-" + f.Shorthand + ", --" + f.Name
		}
		if value := f.Annotations[valueAnnotation]; len(value) > 0 {
			left += " " + value[0]
		}
		rows = append(rows, [2]string{left, f.Usage})
	}
	return rows
}
