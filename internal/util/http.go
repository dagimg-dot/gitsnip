package util

import (
	"net/http"
	"time"
)

const (
	UserAgent      = "GitSnip/1.0"
	DefaultTimeout = 30 * time.Second
)

func NewHTTPClient(token string) *http.Client {
	client := &http.Client{
		Timeout: DefaultTimeout,
	}

	return client
}

func NewGitHubRequest(method, url string, token string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	return req, nil
}
