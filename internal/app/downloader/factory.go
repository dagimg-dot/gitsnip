package downloader

import (
	"fmt"
	"net/http"
	"time"

	"github.com/dagimg-dot/gitsnip/internal/app/gitutil"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
)

func GetDownloader(req model.Request) (Downloader, error) {
	switch req.Method {
	case model.MethodAPI:
		if !req.Source.GitHub() {
			return nil, apperr.Wrap(apperr.ErrUnsupported, nil,
				"The api method only works with github.com", "use --method sparse for other hosts")
		}
		return NewGitHubAPIDownloader(httpClient()), nil
	case model.MethodSparse:
		return NewSparseCheckoutDownloader(gitutil.RealRunner{}), nil
	}
	return nil, fmt.Errorf("unsupported method %q", req.Method)
}

func httpClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{Transport: transport}
}
