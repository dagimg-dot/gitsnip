package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dagimg-dot/gitsnip/internal/app"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
	apperrors "github.com/dagimg-dot/gitsnip/internal/errors"
	"github.com/spf13/cobra"
)

var (
	branch   string
	method   string
	token    string
	provider string
	quiet    bool

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

		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				cmd.Help()
				return nil
			}
			return nil
		},
		Args: cobra.RangeArgs(0, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return nil
			}

			if len(args) < 2 {
				return fmt.Errorf("requires at least repository_url and folder_path arguments")
			}

			repoURL := args[0]
			folderPath := args[1]
			outputDir := "" // default

			if len(args) == 3 {
				outputDir = args[2]
			} else {
				outputDir = filepath.Base(folderPath)
			}

			if provider == "" {
				if strings.Contains(repoURL, "github.com") {
					provider = "github"
				} else {
					provider = "github"
				}
			}

			methodType := model.MethodTypeSparse
			if method == "api" {
				methodType = model.MethodTypeAPI
			}

			providerType := model.ProviderTypeGitHub
			// TODO: add other providers when supported

			opts := model.DownloadOptions{
				RepoURL:   repoURL,
				Subdir:    folderPath,
				OutputDir: outputDir,
				Branch:    branch,
				Token:     token,
				Method:    methodType,
				Provider:  providerType,
				Quiet:     quiet,
			}

			if !quiet {
				fmt.Printf("Repository URL: %s\n", repoURL)
				fmt.Printf("Folder Path:    %s\n", folderPath)
				fmt.Printf("Target Branch:  %s\n", branch)
				fmt.Printf("Download Method: %s\n", method)
				fmt.Printf("Output Dir:     %s\n", outputDir)
				fmt.Printf("Provider:       %s\n", provider)
				fmt.Println("--------------------------------")
			}

			err := app.Download(opts)

			var appErr *apperrors.AppError
			if errors.As(err, &appErr) {
				cmd.SilenceUsage = true
			}

			return err
		},
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main().
func Execute() error {
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = false
	return rootCmd.Execute()
}

// init is called by Go before main()
func init() {
	// TODO: use PersistentFlags if i want flags to be available to subcommands as well
	rootCmd.Flags().StringVarP(&branch, "branch", "b", "main", "Repository branch to download from")
	rootCmd.Flags().StringVarP(&method, "method", "m", "sparse", "Download method ('api' or 'sparse')")
	rootCmd.Flags().StringVarP(&token, "token", "t", "", "GitHub API token for private repositories or increased rate limits")
	rootCmd.Flags().StringVarP(&provider, "provider", "p", "", "Repository provider ('github', more to come)")
	rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress progress output during download")
}
