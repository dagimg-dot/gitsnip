package downloader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/errors"
	"github.com/dagimg-dot/gitsnip/internal/util"
)

const (
	GitHubAPIBaseURL = "https://api.github.com"
)

type GitHubContentItem struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"`
	DownloadURL string `json:"download_url"`
	URL         string `json:"url"`
}

func NewGitHubAPIDownloader(opts model.DownloadOptions) Downloader {
	return &gitHubAPIDownloader{
		opts:   opts,
		client: util.NewHTTPClient(opts.Token),
	}
}

type gitHubAPIDownloader struct {
	opts   model.DownloadOptions
	client *http.Client
}

func (g *gitHubAPIDownloader) Download() error {
	owner, repo, err := parseGitHubURL(g.opts.RepoURL)
	if err != nil {
		return &errors.AppError{
			Err:     errors.ErrInvalidURL,
			Message: "Invalid GitHub URL format",
			Hint:    "URL should be in the format: https://github.com/owner/repo",
		}
	}

	if err := util.EnsureDir(g.opts.OutputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if !g.opts.Quiet {
		fmt.Printf("Downloading directory %s from %s/%s (branch: %s)...\n",
			g.opts.Subdir, owner, repo, g.opts.Branch)
	}

	return g.downloadDirectory(owner, repo, g.opts.Subdir, g.opts.OutputDir)
}

func parseGitHubURL(repoURL string) (owner string, repo string, err error) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`github\.com[/:]([^/]+)/([^/]+?)(?:\.git)?$`),
		regexp.MustCompile(`github\.com[/:]([^/]+)/([^/]+?)(?:\.git)?$`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(repoURL)
		if matches != nil && len(matches) >= 3 {
			return matches[1], matches[2], nil
		}
	}

	return "", "", fmt.Errorf("URL does not match GitHub repository pattern: %s", repoURL)
}

func (g *gitHubAPIDownloader) downloadDirectory(owner, repo, path, outputDir string) error {
	items, err := g.getContents(owner, repo, path)
	if err != nil {
		return err
	}

	for _, item := range items {
		targetPath := filepath.Join(outputDir, item.Name)

		if item.Type == "dir" {
			if err := util.EnsureDir(targetPath); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}

			if err := g.downloadDirectory(owner, repo, item.Path, targetPath); err != nil {
				return err
			}
		} else if item.Type == "file" {
			if !g.opts.Quiet {
				fmt.Printf("Downloading %s\n", item.Path)
			}
			if err := g.downloadFile(item.DownloadURL, targetPath); err != nil {
				return fmt.Errorf("failed to download file %s: %w", item.Path, err)
			}
		}
	}

	return nil
}

func (g *gitHubAPIDownloader) getContents(owner, repo, path string) ([]GitHubContentItem, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s",
		GitHubAPIBaseURL, owner, repo, url.PathEscape(path))

	if g.opts.Branch != "" {
		apiURL = fmt.Sprintf("%s?ref=%s", apiURL, url.QueryEscape(g.opts.Branch))
	}

	req, err := util.NewGitHubRequest("GET", apiURL, g.opts.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, &errors.AppError{
			Err:     errors.ErrNetworkFailure,
			Message: "Failed to connect to GitHub API",
			Hint:    "Check your internet connection and try again",
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := strings.TrimSpace(string(body))
		return nil, errors.ParseGitHubAPIError(resp.StatusCode, bodyStr)
	}

	var items []GitHubContentItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		var item GitHubContentItem
		if errSingle := json.Unmarshal([]byte(err.Error()), &item); errSingle == nil {
			return []GitHubContentItem{item}, nil
		}
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	return items, nil
}

func (g *gitHubAPIDownloader) downloadFile(url, outputPath string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if g.opts.Token != "" {
		req.Header.Set("Authorization", "token "+g.opts.Token)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return &errors.AppError{
			Err:     errors.ErrNetworkFailure,
			Message: "Failed to download file",
			Hint:    "Check your internet connection and try again",
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := strings.TrimSpace(string(body))
		return errors.ParseGitHubAPIError(resp.StatusCode, bodyStr)
	}

	return util.SaveToFile(outputPath, resp.Body)
}
