package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
	"github.com/dagimg-dot/gitsnip/internal/pathspec"
	"github.com/dagimg-dot/gitsnip/internal/util"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

const GitHubAPIBaseURL = "https://api.github.com"

type GitHubContentItem struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"`
	DownloadURL string `json:"download_url"`
}

type gitHubAPIDownloader struct {
	client  HTTPDoer
	baseURL string
}

func NewGitHubAPIDownloader(client HTTPDoer) Downloader {
	return &gitHubAPIDownloader{client: client, baseURL: GitHubAPIBaseURL}
}

type apiDownload struct {
	*gitHubAPIDownloader
	ctx   context.Context
	req   model.Request
	rep   model.Reporter
	owner string
	repo  string
}

func (g *gitHubAPIDownloader) Download(ctx context.Context, req model.Request, dir string, rep model.Reporter) (model.Snapshot, error) {
	owner, repo, err := parseGitHubURL(req.RepoURL)
	if err != nil {
		return model.Snapshot{}, &apperr.Error{
			Err:     apperr.ErrInvalidURL,
			Message: "Invalid GitHub URL format",
			Hint:    "URL should be in the format: https://github.com/owner/repo",
		}
	}

	paths := req.Paths
	if len(paths) == 0 {
		paths = []pathspec.Pattern{{}}
	}

	d := &apiDownload{gitHubAPIDownloader: g, ctx: ctx, req: req, rep: rep, owner: owner, repo: repo}
	for _, p := range paths {
		if p.IsGlob() {
			return model.Snapshot{}, apperr.Wrap(apperr.ErrUnsupported, nil,
				fmt.Sprintf("The api method can't match %q yet", p), "use --method sparse for glob patterns")
		}

		rep.Stage(fmt.Sprintf("Downloading %s from %s/%s...", p, owner, repo))
		items, isFile, err := d.getContents(p.String())
		if err != nil {
			return model.Snapshot{}, err
		}

		target := filepath.Join(dir, filepath.FromSlash(p.String()))
		if isFile {
			err = d.downloadFile(items[0].DownloadURL, target)
		} else {
			err = d.downloadDirectory(items, target)
		}
		if err != nil {
			return model.Snapshot{}, err
		}
	}

	return model.Snapshot{Dir: dir, Ref: req.Ref}, nil
}

func parseGitHubURL(repoURL string) (owner string, repo string, err error) {
	pattern := regexp.MustCompile(`github\.com[/:]([^/]+)/([^/]+?)(?:\.git)?$`)
	matches := pattern.FindStringSubmatch(repoURL)
	if matches != nil && len(matches) >= 3 {
		return matches[1], matches[2], nil
	}

	return "", "", fmt.Errorf("URL does not match GitHub repository pattern: %s", repoURL)
}

func (d *apiDownload) downloadDirectory(items []GitHubContentItem, outputDir string) error {
	if err := util.EnsureDir(outputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for _, item := range items {
		targetPath := filepath.Join(outputDir, item.Name)

		switch item.Type {
		case "dir":
			children, _, err := d.getContents(item.Path)
			if err != nil {
				return err
			}
			if err := d.downloadDirectory(children, targetPath); err != nil {
				return err
			}
		case "file":
			d.rep.Stage("Downloading " + item.Path)
			if err := d.downloadFile(item.DownloadURL, targetPath); err != nil {
				return fmt.Errorf("failed to download file %s: %w", item.Path, err)
			}
		}
	}

	return nil
}

func (d *apiDownload) getContents(path string) ([]GitHubContentItem, bool, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s", d.baseURL, d.owner, d.repo, url.PathEscape(path))
	if d.req.Ref != "" {
		apiURL = fmt.Sprintf("%s?ref=%s", apiURL, url.QueryEscape(d.req.Ref))
	}

	req, err := util.NewGitHubRequest(d.ctx, http.MethodGet, apiURL, d.req.Token)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, false, &apperr.Error{
			Err:     apperr.ErrNetworkFailure,
			Message: "Failed to connect to GitHub API",
			Hint:    "Check your internet connection and try again",
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, false, apperr.ParseGitHubAPIError(resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, false, fmt.Errorf("failed to parse API response: %w", err)
	}

	if len(raw) > 0 && raw[0] == '{' {
		var item GitHubContentItem
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, false, fmt.Errorf("failed to parse API response: %w", err)
		}
		return []GitHubContentItem{item}, item.Type == "file", nil
	}

	var items []GitHubContentItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, false, fmt.Errorf("failed to parse API response: %w", err)
	}
	return items, false, nil
}

func (d *apiDownload) downloadFile(fileURL, outputPath string) error {
	req, err := http.NewRequestWithContext(d.ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	if d.req.Token != "" {
		req.Header.Set("Authorization", "token "+d.req.Token)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return &apperr.Error{
			Err:     apperr.ErrNetworkFailure,
			Message: "Failed to download file",
			Hint:    "Check your internet connection and try again",
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return apperr.ParseGitHubAPIError(resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return util.SaveToFile(outputPath, resp.Body)
}
