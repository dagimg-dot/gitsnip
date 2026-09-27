# gitsnip — Architecture

gitsnip downloads folders, files or glob matches from a remote git repository without cloning it. It has two download engines, a blob-less sparse checkout that works with any git host and a GitHub API engine that works without git, behind one command line whose output is designed for both people and scripts.

---

## Project structure

```
gitsnip/
├── cmd/gitsnip/main.go              # Entry point: signal-aware context, exit code
├── internal/
│   ├── cli/                         # Command line
│   │   ├── root.go                  # Cobra command, flags, request building
│   │   ├── help.go                  # Styled help screen
│   │   ├── output.go                # Summary line and --json result
│   │   ├── errors.go                # Usage errors, error descriptions, exit codes
│   │   ├── token.go                 # --token / GH_TOKEN / GITHUB_TOKEN resolution
│   │   └── version.go               # --version and the hidden version subcommand
│   ├── ui/                          # Terminal renderer
│   │   ├── ui.go                    # Spinner, summary, warnings, errors
│   │   ├── style.go                 # ANSI styles and the plain fallback
│   │   ├── term.go                  # Terminal detection
│   │   ├── term_windows.go          # Enables ANSI processing on Windows consoles
│   │   └── format.go                # Sizes, durations, counts
│   ├── source/source.go             # Parses shorthands, links and remotes
│   ├── pathspec/pathspec.go         # Path and glob patterns, matching, suggestions
│   ├── app/
│   │   ├── app.go                   # Orchestrates a download
│   │   ├── files.go                 # Selects, places and writes the fetched files
│   │   ├── model/types.go           # Request, Snapshot, Result, Reporter, Method
│   │   ├── downloader/
│   │   │   ├── interface.go         # Downloader interface
│   │   │   ├── factory.go           # Picks auto / sparse / api
│   │   │   ├── sparse_checkout.go   # Git engine
│   │   │   ├── git_auth.go          # Token as a host-scoped HTTP header
│   │   │   ├── git_errors.go        # Classifies git failures from stderr
│   │   │   └── github_api.go        # GitHub API engine
│   │   └── gitutil/command.go       # Runs git non-interactively, captures stderr
│   ├── apperr/apperr.go             # Error kinds with user-facing message and hint
│   └── util/fs.go                   # File copies that never follow symlinks
├── .github/workflows/
│   ├── ci.yml                       # gofmt, vet, race tests, cross-builds on PRs
│   └── release.yml                  # GoReleaser on version tags
├── .goreleaser.yml
└── Makefile
```

---

## How a download flows

```
gitsnip <source> [path...] [flags]
   │
   ▼
cli ── source.Parse(<source>) ─────────── owner/repo, link, SSH remote, file:// URL
   │   pathspec.ParseAll(paths) ───────── folders, files, globs
   │   model.Request{Source, Ref, Paths, Output, Token, Method, Force}
   ▼
app.Download
   │   downloader.GetDownloader ───────── auto → sparse if git exists, else api for GitHub
   │   staging := temp dir
   │   Downloader.Download(ctx, req, staging, reporter) → Snapshot{Dir, Ref, Commit, Paths, List}
   │   list files in the snapshot, select those matching the paths
   │   choose the output folder and check for clashes
   │   copy the selection, recreating safe symlinks and skipping the rest
   ▼
Result{Method, Ref, Commit, Paths, Output, Target, Files, Bytes}
   │
   ▼
cli ── ui.Success (one summary line) or writeJSON (stdout)
```

Downloaders never print and never touch the output folder. They fill a staging directory, report progress through `model.Reporter`, and return a `Snapshot`. Everything about where files land lives in `app`.

---

## Sources

`source.Parse` turns user input into a `Source{Host, Owner, Repo, URL, Ref, RefPath, Path}`:

- `owner/repo[/path][@ref]` is a GitHub shorthand.
- `host/owner/repo...` without a scheme is treated as HTTPS.
- Browser links are recognized by their markers: GitHub `tree|blob|commit|releases/tag`, GitLab `/-/`, Gitea/Forgejo `src/branch|tag|commit`, Bitbucket `src`, and sourcehut `tree/<ref>/item`.
- Unknown hosts without markers treat the whole path as the repository, which suits nested GitLab groups.
- `git@host:path` and `ssh://` remotes, plus `file://` URLs, are passed to git unchanged.

A link like `tree/feature/login/src` can't tell where the ref ends and the path begins, so the parser keeps it as `RefPath`. Each engine resolves it against the remote: the git engine uses one `ls-remote` call over all candidate prefixes, and the API engine tries candidates shortest first.

---

## Engines

### Sparse (git)

1. `git clone --depth=1 --filter=blob:none --no-checkout [--branch ref] -- <url>` fetches commits and trees but no file contents. A full commit SHA instead goes through `init` plus a partial `fetch` of that commit.
2. Non-cone sparse rules (`/path`, `/data/*.json`) are written to `.git/info/sparse-checkout`.
3. `git read-tree -mu HEAD` checks out the matching files, and git fetches only their blobs, in one batch.
4. `rev-parse` and `symbolic-ref` record the commit and the default branch name. `ls-tree` serves path suggestions from the local trees.

Git runs with `GIT_TERMINAL_PROMPT=0` (it never prompts), `LC_ALL=C` (its errors stay parseable) and a low-speed limit (a stalled transfer aborts after 60 seconds under 1 KB/s). Tokens are passed through `GIT_CONFIG_*` variables as an `http.<host>.extraheader`, so they appear neither in argv nor in `.git/config`. `--` guards every URL argument.

### API (GitHub)

1. `GET /repos/{o}/{r}` finds the default branch, and `GET /repos/{o}/{r}/commits/{ref}` resolves the ref to a SHA.
2. `GET /repos/{o}/{r}/git/trees/{sha}?recursive=1` lists the whole tree in one call. A truncated tree is refused with a hint to use git.
3. Matching files download from `raw.githubusercontent.com/{o}/{r}/{sha}/{path}` with eight workers. Executable bits are kept, symlink targets come from the blobs API, and submodules are skipped with a warning.

Rate-limit responses report when the limit resets.

---

## Writing the output

`app/files.go` lists every regular file and symlink in the snapshot, skipping `.git`, and matches it against the patterns. A pattern that matches nothing becomes a "doesn't exist" error with a suggestion: an exact match ignoring case, the same name elsewhere, or the closest by edit distance.

Files are written relative to the common parent of the patterns' anchors:

| Request | Output |
|---|---|
| one folder `src/components` | `./components/…` |
| one file `src/a.txt` | `./a.txt` |
| `data/usage.txt 'data/*.json'` | `./data/…` |
| nothing (whole repository) | `./<repo>/…` |

Before anything is written, clashes with existing files abort the download unless `--force` is set. If writing fails midway, an output folder the run created is removed again. Symlinks are never followed: a link whose target stays inside the downloaded folder is recreated as a link, and anything absolute or escaping is skipped with a warning.

---

## Terminal output

`internal/ui` writes everything human-facing to stderr. Stdout is reserved for `--json`.

- On an interactive terminal, a braille spinner shows the current stage and progress, and is replaced in place by the summary line.
- Colors are six ANSI styles (bold, dim, red, green, yellow, cyan), so they follow the user's terminal theme.
- Styling is used only when stderr is a terminal and `TERM` isn't `dumb`. `NO_COLOR` turns off colors but keeps the spinner.
- On Windows, ANSI processing is switched on first. If that fails, output is plain.
- Every styled write is attempted, and if it returns an error or panics, the renderer permanently switches to plain text and writes the same line again without escape codes.
- `-v` adds one line per git command or HTTP request with its timing, and error causes such as git's raw stderr.

---

## Errors

`apperr.Error` carries a kind (`ErrRefNotFound`, `ErrPathNotFound`, `ErrRateLimitExceeded`, …), a message, a hint and the underlying cause. `errors.Is` matches both the kind and the cause.

- Git failures are classified from git's stderr in `downloader/git_errors.go`.
- GitHub API failures are classified from the status code and rate-limit headers.
- The CLI renders `✗ message` and `→ hint`, adds `; drop -b to use it` when the branch came from `-b`, and maps errors to exit codes: `2` for usage, `1` for failures, `130` for Ctrl-C.

---

## Testing

- Unit tests sit next to the code: source parsing, path matching, the renderer (including the plain fallback) and file placement.
- The git engine and the CLI are tested end to end against real git repositories created in temporary directories and served over `file://`, with `uploadpack.allowFilter` enabled so partial clones behave as they do on GitHub.
- The API engine is tested against a fake GitHub built with `httptest`.

Run `make test` and `go vet ./...` before committing. CI runs both with `-race` on Linux and macOS.

---

## Build and release

| Variable | Source |
|---|---|
| `version` | `git describe --tags` (Makefile) or the tag (GoReleaser) |
| `commit` | short commit hash |
| `buildDate` | build time |
| `builtBy` | `whoami` or `goreleaser` |

They are injected into `internal/cli` with `-ldflags -X`. Builds from `go install …@version` fall back to the module version from the build info. GoReleaser publishes static binaries for Linux, macOS and Windows on amd64 and arm64 when a `v*` tag is pushed.
