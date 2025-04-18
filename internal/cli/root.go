package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"path/filepath"
)

var (
	branch string
	method string

	rootCmd = &cobra.Command{
		Use:   "gitsnip <repository_url> <folder_path> [output_dir]",
		Short: "Download a specific folder from a Git repository (GitHub)",
		Long: `Gitsnip allows you to download a specific folder from a remote Git
repository without cloning the entire repository.

Arguments:
  repository_url: URL of the GitHub repository (e.g., https://github.com/user/repo)
  folder_path:    Path to the folder within the repository you want to download.
  output_dir:     Optional. Directory where the folder should be saved.
                  Defaults to the folder's base name in the current directory.`,

		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoURL := args[0]
			folderPath := args[1]
			outputDir := "" // default

			if len(args) == 3 {
				outputDir = args[2]
			} else {
				outputDir = filepath.Base(folderPath)
			}

			fmt.Printf("Repository URL: %s\n", repoURL)
			fmt.Printf("Folder Path:    %s\n", folderPath)
			fmt.Printf("Target Branch:  %s\n", branch)
			fmt.Printf("Download Method: %s\n", method)
			fmt.Printf("Output Dir:     %s\n", outputDir)

			return nil
		},
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main().
func Execute() error {
	return rootCmd.Execute()
}

// init is called by Go before main()
func init() {
	// TODO: use PersistentFlags if i want flags to be available to subcommands as well
	rootCmd.Flags().StringVarP(&branch, "branch", "b", "main", "Repository branch to download from")
	rootCmd.Flags().StringVarP(&method, "method", "m", "sparse", "Download method ('api' or 'sparse')")
}
