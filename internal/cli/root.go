package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dagimg-dot/gitsnip/internal/app"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
	"github.com/dagimg-dot/gitsnip/internal/pathspec"
	"github.com/dagimg-dot/gitsnip/internal/source"
	"github.com/spf13/cobra"
)

var (
	branch   string
	method   string
	token    string
	provider string
	quiet    bool
	force    bool

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

			src, err := source.Parse(args[0])
			if err != nil {
				return apperr.Wrap(apperr.ErrInvalidURL, err, err.Error(), "use owner/repo, a GitHub or GitLab link, or a git URL")
			}

			var raws []string
			if src.Path != "" {
				raws = append(raws, src.Path)
			}
			if len(args) >= 2 {
				raws = append(raws, args[1])
			}
			patterns, err := pathspec.ParseAll(raws)
			if err != nil {
				return err
			}

			output := ""
			outputDir := "(automatic)"
			if len(args) == 3 {
				output = args[2]
				outputDir = output
			}

			ref := branch
			switch {
			case ref == "":
				ref = src.Ref
			case src.RefPath != "" || (src.Ref != "" && src.Ref != ref):
				return fmt.Errorf("the source already names a branch; drop -b")
			}

			methodType := model.MethodSparse
			if method == "api" {
				methodType = model.MethodAPI
			}

			req := model.Request{
				Source: src,
				Ref:    ref,
				Paths:  patterns,
				Output: output,
				Token:  token,
				Method: methodType,
				Force:  force,
			}

			if !quiet {
				fmt.Printf("Repository URL: %s\n", src.URL)
				fmt.Printf("Folder Path:    %s\n", strings.Join(raws, ", "))
				shownBranch := branch
				if shownBranch == "" {
					shownBranch = "(default)"
				}
				fmt.Printf("Target Branch:  %s\n", shownBranch)
				fmt.Printf("Download Method: %s\n", method)
				fmt.Printf("Output Dir:     %s\n", outputDir)
				fmt.Println("--------------------------------")
			}

			var rep model.Reporter = model.Discard{}
			if !quiet {
				rep = linePrinter{}
			}

			_, err = app.Download(cmd.Context(), req, rep)
			var appErr *apperr.Error
			if errors.As(err, &appErr) {
				cmd.SilenceUsage = true
			}
			if err == nil && !quiet {
				fmt.Println("Download completed successfully.")
			}

			return err
		},
	}
)

type linePrinter struct{}

func (linePrinter) Stage(text string) { fmt.Println(text) }
func (linePrinter) Progress(int, int) {}
func (linePrinter) Warn(text string)  { fmt.Println("Warning: " + text) }
func (linePrinter) Debug(text string) {}

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
	rootCmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch, tag or commit to download from (default: the repository's default branch)")
	rootCmd.Flags().StringVarP(&method, "method", "m", "sparse", "Download method ('api' or 'sparse')")
	rootCmd.Flags().StringVarP(&token, "token", "t", "", "GitHub API token for private repositories or increased rate limits")
	rootCmd.Flags().StringVarP(&provider, "provider", "p", "", "Repository provider ('github', more to come)")
	rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress progress output during download")
	rootCmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite files that already exist")
}
