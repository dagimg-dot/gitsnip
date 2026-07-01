# gitsnip — Architecture

gitsnip is a CLI tool that downloads a specific folder from a remote Git repository (GitHub) without cloning the entire repository. It offers two download strategies — GitHub REST API and Git sparse checkout — selectable via a CLI flag.

---

## Project Structure

```
gitsnip/
├── cmd/gitsnip/
│   └── main.go                     # Entry point
├── internal/
│   ├── cli/
│   │   ├── root.go                 # Cobra root command + flag wiring
│   │   └── version.go              # `version` subcommand
│   ├── app/
│   │   ├── app.go                  # Orchestrator — resolves and invokes downloader
│   │   ├── model/
│   │   │   └── types.go            # Domain types (DownloadOptions, MethodType, ProviderType)
│   │   ├── downloader/
│   │   │   ├── interface.go        # Downloader interface
│   │   │   ├── factory.go          # Downloader factory (strategy selection)
│   │   │   ├── github_api.go       # API-based downloader
│   │   │   └── sparse_checkout.go  # Git sparse-checkout downloader
│   │   └── gitutil/
│   │       └── command.go          # Git command execution, temp dir helpers
│   ├── errors/
│   │   └── errors.go               # AppError type, sentinel errors, parse helpers
│   └── util/
│       ├── fs.go                   # Filesystem operations (copy, save, mkdir)
│       └── http.go                 # HTTP client, GitHub request builder
├── assets/
│   └── gitsnip-showcase.gif        # Demo animation
├── .github/workflows/release.yml   # GoReleaser CI/CD
├── .goreleaser.yml                 # Cross-platform release config
├── Makefile                        # Build/lint/release targets
├── go.mod / go.sum
├── README.md
└── LICENSE
```

---

## Layer Architecture

### 1. Entry Point — `cmd/gitsnip/main.go`

Minimal. Calls `cli.Execute()`, formats errors via `errors.FormatError()`, exits with code 1 on failure.

### 2. CLI Layer — `internal/cli/`

Built on [spf13/cobra](https://github.com/spf13/cobra).

- **root.go** — Defines the primary command:
  ```
  gitsnip <repository_url> <folder_path> [output_dir]
  ```
  Parses positional args, binds flags (`--branch`, `--method`, `--token`, `--provider`, `--quiet`), constructs a `model.DownloadOptions`, delegates to `app.Download()`, and handles error display (suppressing usage on `AppError`).

- **version.go** — Subcommand `gitsnip version`. Displays build-time injected vars (`version`, `commit`, `buildDate`, `builtBy`), set via linker flags (`-X`).

### 3. Application Layer — `internal/app/`

Contains all business logic, split into model, downloader strategies, and git utilities.

#### 3a. Domain Model — `internal/app/model/types.go`

| Type | Values |
|---|---|
| `MethodType` | `"sparse"`, `"api"` |
| `ProviderType` | `"github"` (extensible) |

`DownloadOptions` is the single config struct flowing from CLI through to the downloader:

```go
type DownloadOptions struct {
    RepoURL, Subdir, OutputDir, Branch, Token string
    Method    MethodType
    Provider  ProviderType
    Quiet     bool
}
```

#### 3b. Orchestrator — `internal/app/app.go`

```go
func Download(opts model.DownloadOptions) error {
    dl, err := downloader.GetDownloader(opts)
    return dl.Download()
}
```

Single function — resolves a downloader from the factory and runs it.

#### 3c. Downloader Interface & Factory — `internal/app/downloader/`

**Interface** (`interface.go`):

```go
type Downloader interface {
    Download() error
}
```

**Factory** (`factory.go`):

```
GetDownloader(opts)
  ├── MethodTypeAPI + ProviderTypeGitHub → NewGitHubAPIDownloader(opts)
  └── MethodTypeSparse                    → NewSparseCheckoutDownloader(opts)
```

`MethodTypeAPI` delegates to provider-based constructors (currently only GitHub). `MethodTypeSparse` is provider-agnostic and works with any git URL.

#### 3d. GitHub API Downloader — `internal/app/downloader/github_api.go`

Uses the [GitHub Contents API](https://docs.github.com/en/rest/repos/contents) to list and download files recursively.

**Flow:**

1. `parseGitHubURL(repoURL)` → owner, repo — regex-based (handles `github.com/owner/repo` and `github.com:owner/repo.git`)
2. `getContents(owner, repo, path)` → `GET /repos/{owner}/{repo}/contents/{path}?ref={branch}` — returns directory listing or single file info
3. `downloadDirectory(owner, repo, path, outputDir)` — recursively walks the tree:
   - Item is `"dir"` → create local dir, recurse
   - Item is `"file"` → download from `download_url`, `SaveToFile`
4. `downloadFile(url, outputPath)` — fetches raw file content, writes to disk

Authentication via `Authorization: token` header; user-agent set to `GitSnip/1.0`.

**Limitations:** GitHub API rate limits (60/hr unauthenticated, 5000/hr with token); recursive directory walking means N+1 requests.

#### 3e. Sparse Checkout Downloader — `internal/app/downloader/sparse_checkout.go`

Uses Git's sparse-checkout feature to fetch only the target directory.

**Flow:**

1. Check `git` is installed (`gitutil.IsGitInstalled()`)
2. Create temp dir (`gitutil.CreateTempDir`, cleaned via `defer gitutil.CleanupTempDir`)
3. `getAuthenticatedRepoURL()` — embeds token into URL for authenticated cloning: `https://TOKEN@github.com/owner/repo`
4. `initRepo` — `git init` + `git remote add origin <url>`
5. `setupSparseCheckout` — `git sparse-checkout init --cone` + `git sparse-checkout set <subdir>`
6. `pullContent` — `git fetch --depth=1 --no-tags origin <branch>` + `git checkout FETCH_HEAD`
7. Verify requested path exists in temp repo
8. `util.CopyDirectory(sparsePath, outputDir)` — copy result to final destination

All git operations run with a 2-minute context timeout.

**Advantages:** Single network round-trip (one fetch), no rate limits, works with any git host. **Requirement:** Git installed.

### 4. Git Utilities — `internal/app/gitutil/command.go`

| Function | Purpose |
|---|---|
| `RunGitCommand(ctx, dir, args...)` | Execute git command with stdout/stderr capture |
| `RunGitCommandWithInput(ctx, dir, input, args...)` | Same with stdin pipe |
| `IsGitInstalled()` | Existence check via `exec.LookPath("git")` |
| `GitVersion()` | Returns `git --version` output |
| `CreateTempDir()` / `CleanupTempDir(dir)` | Temp directory lifecycle (pattern: `gitsnip-*`) |

Default command timeout is 60s when no context is provided.

### 5. Error Layer — `internal/errors/errors.go`

**`AppError`** struct wraps errors with user-facing hints:

```go
type AppError struct {
    Err        error   // underlying (sentinel or raw)
    Message    string  // user-facing message
    Hint       string  // actionable suggestion
    StatusCode int     // HTTP status code (API errors)
}
```

**Sentinel errors** (all package-level `var`):

| Error | Context |
|---|---|
| `ErrRateLimitExceeded` | GitHub API rate limit |
| `ErrAuthenticationRequired` | Private repo / bad token |
| `ErrRepositoryNotFound` | 404 from API or git |
| `ErrPathNotFound` | Subdirectory missing in repo |
| `ErrNetworkFailure` | Connection errors |
| `ErrInvalidURL` | Malformed repository URL |
| `ErrGitNotInstalled` | Git missing on system |
| `ErrGitCommandFailed` | Generic git failure |
| `ErrGitCloneFailed` | Clone failure |
| `ErrGitFetchFailed` | Fetch failure |
| `ErrGitCheckoutFailed` | Checkout failure |
| `ErrGitInvalidRepository` | Invalid git repo |

**Parse functions:**

- `ParseGitHubAPIError(statusCode, body)` — maps HTTP 401/403/404 to appropriate sentinel + hint
- `ParseGitError(err, stderr)` — parses git stderr for known error patterns

**`FormatError(err)`** — formats `AppError` as `"message\nHint: ..."`, falls back to `"%v"` for other errors.

### 6. Utilities — `internal/util/`

**`fs.go`**:

| Function | Purpose |
|---|---|
| `EnsureDir(path)` | `os.MkdirAll` with 0755 |
| `FileExists(path)` | Stat check |
| `SaveToFile(path, reader)` | Create parent dirs, write from reader |
| `CopyDirectory(src, dst)` | Recursive dir copy (preserves permissions) |
| `CopyFile(src, dst)` | Single file copy (preserves permissions) |

**`http.go`**:

- `NewHTTPClient(token)` — `http.Client` with 30s timeout
- `NewGitHubRequest(method, url, token)` — creates request with User-Agent, GitHub API Accept header, optional `Authorization: token`

---

## Data Flow Diagram

```
Terminal
   │
   ▼
cmd/gitsnip/main.go
   │  cli.Execute()
   ▼
internal/cli/root.go
   │  Parse args + flags → model.DownloadOptions
   │  app.Download(opts)
   ▼
internal/app/app.go
   │  downloader.GetDownloader(opts)
   ▼
internal/app/downloader/factory.go
   │
   ├─── MethodTypeAPI ────► github_api.go
   │                          │
   │                          ├── parseGitHubURL → owner, repo
   │                          ├── GET /repos/{owner}/{repo}/contents/{path}?ref={branch}
   │                          ├── Recursive walk (dir → recurse, file → download)
   │                          └── SaveToFile (each file)
   │
   └─── MethodTypeSparse ──► sparse_checkout.go
                              │
                              ├── git init
                              ├── git remote add origin <url>
                              ├── git sparse-checkout init --cone
                              ├── git sparse-checkout set <subdir>
                              ├── git fetch --depth=1 origin <branch>
                              ├── git checkout FETCH_HEAD
                              └── CopyDirectory → output
```

---

## Configuration & Build

### Build Variables (linker flags)

| Variable | Source |
|---|---|
| `version` | `git describe --tags` |
| `commit` | `git rev-parse --short HEAD` |
| `buildDate` | `date -u +"%Y-%m-%dT%H:%M:%SZ"` |
| `builtBy` | `whoami` |

Injected via `-ldflags` in Makefile and `.goreleaser.yml`.

### Makefile Targets

| Target | Action |
|---|---|
| `build` | Build binary to `bin/gitsnip` |
| `run` | `go run` with passthrough args |
| `run-build` | Build then run binary |
| `clean` | Remove `bin/` and `dist/` |
| `lint` | `go fmt ./...` |
| `release` | Git tag + push |
| `local-release` | Build single-platform binary to `dist/` |

### Cross-Platform Releases

GoReleaser config builds for:
- **OS**: Linux, Windows, macOS
- **Arch**: amd64, arm64
- **Archive**: `.tar.gz` (Linux/macOS), `.zip` (Windows)
- CGO disabled for static binaries.

GitHub Actions triggers on `v*` tags.

---

## Design Patterns

| Pattern | Usage |
|---|---|
| **Strategy** | `Downloader` interface with `APIDownloader` and `SparseCheckoutDownloader` implementations |
| **Factory Method** | `GetDownloader()` in `factory.go` selects strategy by method/provider |
| **Value Object** | `MethodType`, `ProviderType` as typed string constants |
| **Result Object** | `AppError` carrying message, hint, status code, and cause |
| **Separated Interface** | CLI (`internal/cli`) separated from domain (`internal/app`) |

---

## Extending for New Providers

1. Add a `ProviderType` constant in `model/types.go`
2. Implement a new downloader (e.g., `gitlab_api.go`) satisfying the `Downloader` interface
3. Add a case in `factory.go`'s provider switch
4. Optionally update the provider auto-detection in `root.go`

---

## Error Handling Strategy

- **Sentinel errors** for programmatic comparison (callers can `errors.Is(err, ErrRateLimitExceeded)`)
- **`AppError`** carries user-facing message + hint, used by `FormatError` for pretty-printing
- **Parse functions** translate external error formats (HTTP JSON, git stderr) into `AppError` with context-appropriate hints
- **Usage suppression**: If error is an `AppError` (meaning the user's input was wrong), `cmd.SilenceUsage = true` to avoid repeating the help text
