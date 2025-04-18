package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
	builtBy   = "unknown"

	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Long:  `Display version, build, and other information about GitSnip.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("GitSnip %s\n", version)
			fmt.Printf("  Commit: %s\n", commit)
			fmt.Printf("  Built on: %s\n", buildDate)
			fmt.Printf("  Built by: %s\n", builtBy)
		},
	}
)

func init() {
	rootCmd.AddCommand(versionCmd)
}
