package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dagimg-dot/gitsnip/internal/app"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/pathspec"
	"github.com/dagimg-dot/gitsnip/internal/source"
	"github.com/dagimg-dot/gitsnip/internal/ui"
	"github.com/spf13/cobra"
)

type options struct {
	output   string
	branch   string
	method   string
	token    string
	provider string
	force    bool
	quiet    bool
	verbose  bool
	json     bool
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var o options
	cmd := newRootCmd(&o, stdout, stderr)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(ctx)
	if err == nil {
		return 0
	}

	message, hint, detail := describe(err, o.branch != "")
	ui.New(stderr, ui.Options{Verbose: o.verbose}).Fail(message, hint, detail)
	return exitCode(err)
}

func newRootCmd(o *options, stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "gitsnip <source> [path...]",
		Short:         "Download folders and files from any git repository",
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       currentVersion(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return snip(cmd.Context(), o, args, stdout, stderr)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetVersionTemplate("gitsnip {{.Version}}\n")
	cmd.SetHelpFunc(func(c *cobra.Command, _ []string) { writeHelp(c, stdout) })
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usage(capitalize(err.Error()), "run gitsnip --help for usage")
	})
	cmd.CompletionOptions.HiddenDefaultCmd = true

	f := cmd.Flags()
	f.StringVarP(&o.output, "output", "o", "", "where to write (default: the folder's name)")
	f.StringVarP(&o.branch, "branch", "b", "", "branch, tag or commit (default: the repo's default)")
	f.StringVarP(&o.method, "method", "m", "auto", "auto, sparse or api (default auto)")
	f.StringVarP(&o.token, "token", "t", "", "access token (default: $GH_TOKEN or $GITHUB_TOKEN)")
	f.BoolVarP(&o.force, "force", "f", false, "overwrite existing files")
	f.BoolVarP(&o.quiet, "quiet", "q", false, "print nothing on success")
	f.BoolVarP(&o.verbose, "verbose", "v", false, "show git commands and API calls")
	f.BoolVar(&o.json, "json", false, "print the result as JSON")
	f.BoolP("help", "h", false, "show this help")
	f.Bool("version", false, "print the version")
	f.StringVarP(&o.provider, "provider", "p", "", "")
	f.MarkHidden("provider")
	for name, value := range map[string]string{"output": "dir", "branch": "ref", "method": "name", "token": "token"} {
		f.SetAnnotation(name, valueAnnotation, []string{value})
	}

	cmd.AddCommand(newVersionCmd(stdout))
	return cmd
}

func snip(ctx context.Context, o *options, args []string, stdout, stderr io.Writer) error {
	started := time.Now()
	u := ui.New(stderr, ui.Options{Quiet: o.quiet, Verbose: o.verbose, JSON: o.json})
	defer u.Stop()

	req, err := buildRequest(o, args, u)
	if err != nil {
		return err
	}

	u.Start(req.Source.Display())
	res, err := app.Download(ctx, &req, u)
	if err != nil {
		return err
	}

	elapsed := time.Since(started)
	if o.json {
		return writeJSON(stdout, &req.Source, &res, u.Warnings(), elapsed)
	}
	u.Success(summarize(&req.Source, &res, elapsed))
	return nil
}

func buildRequest(o *options, args []string, u *ui.UI) (model.Request, error) {
	if o.provider != "" {
		u.Warn("--provider isn't needed anymore; the host comes from the source")
	}

	src, err := source.Parse(args[0])
	if err != nil {
		return model.Request{}, usage(capitalize(err.Error()), "use owner/repo, a link to a repository, folder or file, or a git URL")
	}

	raws := args[1:]
	output := o.output
	if output == "" && len(raws) >= 2 && looksLocal(raws[len(raws)-1]) {
		output, raws = raws[len(raws)-1], raws[:len(raws)-1]
		u.Warn("positional output folders are deprecated; use -o " + output)
	}
	if src.Path != "" {
		raws = append([]string{src.Path}, raws...)
	}
	paths, err := pathspec.ParseAll(raws)
	if err != nil {
		return model.Request{}, usage(capitalize(err.Error()), "")
	}

	ref, err := pickRef(&src, o.branch)
	if err != nil {
		return model.Request{}, err
	}

	method, err := model.ParseMethod(o.method)
	if err != nil {
		return model.Request{}, usage(capitalize(err.Error()), "")
	}

	return model.Request{
		Source: src,
		Ref:    ref,
		Paths:  paths,
		Output: output,
		Token:  resolveToken(o.token, &src, os.Getenv),
		Method: method,
		Force:  o.force,
	}, nil
}

func pickRef(src *source.Source, branch string) (string, error) {
	switch {
	case branch == "":
		return src.Ref, nil
	case src.RefPath != "":
		return "", usage("The link already names a branch", "drop -b, or link to the branch you want")
	case src.Ref != "" && src.Ref != branch:
		return "", usage(fmt.Sprintf("The source asks for %q but -b asks for %q", src.Ref, branch), "keep one of them")
	}
	return branch, nil
}

func looksLocal(arg string) bool {
	if arg == "." || arg == ".." || filepath.IsAbs(arg) || filepath.VolumeName(arg) != "" || strings.HasPrefix(arg, "~") {
		return true
	}
	for _, prefix := range []string{"./", "../", `.\`, `..\`} {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}
