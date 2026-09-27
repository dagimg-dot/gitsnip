package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/app/gitutil"
	"github.com/dagimg-dot/gitsnip/internal/cli"
)

func gitsnip(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := cli.Run(context.Background(), args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func fixture(t *testing.T) string {
	t.Helper()
	if !gitutil.IsGitInstalled() {
		t.Skip("git is not installed")
	}

	root := t.TempDir()
	work := filepath.Join(root, "work")
	bare := filepath.Join(root, "repo.git")
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	files := map[string]string{
		"README.md":                     "# fixture\n",
		"src/components/Button.tsx":     "export const Button = () => null\n",
		"src/components/icons/star.svg": "<svg/>\n",
	}
	for name, content := range files {
		full := filepath.Join(work, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(content), 0o644)
	}
	git(root, "init", "--quiet", "--initial-branch=trunk", work)
	git(work, "add", "-A")
	git(work, "commit", "--quiet", "-m", "init")
	git(root, "init", "--quiet", "--bare", bare)
	git(bare, "config", "uploadpack.allowFilter", "true")
	git(work, "push", "--quiet", bare, "trunk")
	git(bare, "symbolic-ref", "HEAD", "refs/heads/trunk")

	t.Chdir(t.TempDir())
	return "file://" + filepath.ToSlash(bare)
}

func TestNoArgumentsShowsTheHelp(t *testing.T) {
	code, stdout, stderr := gitsnip(t)
	if code != 0 || stderr != "" {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	for _, want := range []string{
		"Usage\n  gitsnip <source> [path...] [flags]\n",
		"  $ gitsnip https://github.com/owner/repo/tree/main/docs\n",
		"  -o, --output dir    where to write (default: the folder's name)\n",
		"      --json          print the result as JSON\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help is missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "provider") || strings.Contains(stdout, "completion") {
		t.Errorf("help shows hidden things:\n%s", stdout)
	}
}

func TestVersion(t *testing.T) {
	if code, stdout, _ := gitsnip(t, "--version"); code != 0 || stdout != "gitsnip dev\n" {
		t.Errorf("--version: code %d, %q", code, stdout)
	}
	if code, stdout, _ := gitsnip(t, "version"); code != 0 || stdout != "gitsnip dev\n" {
		t.Errorf("version: code %d, %q", code, stdout)
	}
}

func TestUsageErrorsExitWithTwoAndNoUsageDump(t *testing.T) {
	cases := map[string][]string{
		"✗ Unknown flag: --bogus\n  → run gitsnip --help for usage\n":                      {"o/r", "--bogus"},
		"✗ \"gitsnip\" isn't owner/repo, a link or a git remote\n":                         {"gitsnip"},
		"✗ Unknown method \"bogus\", use auto, sparse or api\n":                            {"o/r", "-m", "bogus"},
		"✗ The source asks for \"v1\" but -b asks for \"v2\"\n  → keep one of them\n":      {"o/r@v1", "-b", "v2"},
		"✗ The link already names a branch\n  → drop -b, or link to the branch you want\n": {"https://github.com/o/r/tree/main/x", "-b", "dev"},
	}
	for want, args := range cases {
		code, stdout, stderr := gitsnip(t, args...)
		if code != 2 || stdout != "" {
			t.Errorf("%v: code %d stdout %q", args, code, stdout)
		}
		if !strings.HasPrefix(stderr, strings.SplitN(want, "\n", 2)[0]) || !strings.Contains(stderr, want) {
			t.Errorf("%v:\n got %q\nwant %q", args, stderr, want)
		}
	}
}

func TestDownloadsAFolderWithOneSummaryLine(t *testing.T) {
	repo := fixture(t)
	code, stdout, stderr := gitsnip(t, repo, "src/components")
	if code != 0 || stdout != "" {
		t.Fatalf("code %d stdout %q stderr %q", code, stdout, stderr)
	}
	line := regexp.MustCompile(`^✓ repo@trunk · src/components → \./components   2 files · 40 B · \d+\.\ds\n$`)
	if !line.MatchString(stderr) {
		t.Errorf("summary = %q", stderr)
	}
	if _, err := os.Stat(filepath.Join("components", "icons", "star.svg")); err != nil {
		t.Error(err)
	}
}

func TestJSONGoesToStdout(t *testing.T) {
	repo := fixture(t)
	code, stdout, stderr := gitsnip(t, repo, "src/components/Button.tsx", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	var got struct {
		Repo, Ref, Method, Output string
		Paths                     []string
		Files                     int
		Bytes                     int64
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout isn't JSON: %v\n%s", err, stdout)
	}
	cwd, _ := os.Getwd()
	if got.Repo != "repo" || got.Ref != "trunk" || got.Method != "sparse" || got.Files != 1 || got.Bytes != 33 ||
		len(got.Paths) != 1 || got.Paths[0] != "src/components/Button.tsx" || got.Output != cwd {
		t.Errorf("json = %+v", got)
	}
}

func TestQuietPrintsNothing(t *testing.T) {
	repo := fixture(t)
	if code, stdout, stderr := gitsnip(t, repo, "src", "-q"); code != 0 || stdout != "" || stderr != "" {
		t.Errorf("code %d stdout %q stderr %q", code, stdout, stderr)
	}
}

func TestMissingPathsSuggestTheClosestOne(t *testing.T) {
	repo := fixture(t)
	code, _, stderr := gitsnip(t, repo, "src/componets")
	want := "✗ Path \"src/componets\" doesn't exist in repo@trunk\n  → did you mean src/components?\n"
	if code != 1 || stderr != want {
		t.Errorf("code %d\n got %q\nwant %q", code, stderr, want)
	}
}

func TestMissingBranchPointsAtTheDefaultAndTheFlag(t *testing.T) {
	repo := fixture(t)
	code, _, stderr := gitsnip(t, repo, "src", "-b", "main")
	want := "✗ Branch or tag \"main\" doesn't exist in repo\n  → the default branch is \"trunk\"; drop -b to use it\n"
	if code != 1 || stderr != want {
		t.Errorf("code %d\n got %q\nwant %q", code, stderr, want)
	}
}

func TestPositionalOutputStillWorksWithAWarning(t *testing.T) {
	repo := fixture(t)
	code, _, stderr := gitsnip(t, repo, "src/components", "./out")
	if code != 0 || !strings.HasPrefix(stderr, "! positional output folders are deprecated; use -o ./out\n✓ ") {
		t.Errorf("code %d stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join("out", "Button.tsx")); err != nil {
		t.Error(err)
	}
}

func TestRefusesToOverwriteUnlessForced(t *testing.T) {
	repo := fixture(t)
	gitsnip(t, repo, "src/components", "-q")
	code, _, stderr := gitsnip(t, repo, "src/components")
	want := "✗ ." + string(filepath.Separator) + "components already has 2 of these files\n  → pass --force to overwrite, or -o to write somewhere else\n"
	if code != 1 || stderr != want {
		t.Errorf("code %d\n got %q\nwant %q", code, stderr, want)
	}
	if code, _, stderr := gitsnip(t, repo, "src/components", "--force", "-q"); code != 0 {
		t.Errorf("--force: code %d stderr %q", code, stderr)
	}
}

func TestProviderFlagIsAcceptedButIgnored(t *testing.T) {
	repo := fixture(t)
	code, _, stderr := gitsnip(t, repo, "README.md", "-p", "github")
	if code != 0 || !strings.HasPrefix(stderr, "! --provider isn't needed anymore; the host comes from the source\n✓ repo@trunk · README.md → ./README.md") {
		t.Errorf("code %d stderr %q", code, stderr)
	}
}

func TestVerboseShowsGitCommands(t *testing.T) {
	repo := fixture(t)
	code, _, stderr := gitsnip(t, repo, "README.md", "-v")
	if code != 0 || !strings.Contains(stderr, "· cloning\n") || !strings.Contains(stderr, "  git clone --quiet --depth=1 --filter=blob:none") {
		t.Errorf("code %d stderr %q", code, stderr)
	}
}
