package downloader

import (
	"fmt"
	"net/http"
	"time"

	"github.com/dagimg-dot/gitsnip/internal/app/gitutil"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
)

func GetDownloader(req *model.Request) (Downloader, model.Method, error) {
	method := req.Method
	if method == "" || method == model.MethodAuto {
		switch {
		case gitutil.IsGitInstalled():
			method = model.MethodSparse
		case req.Source.GitHub():
			method = model.MethodAPI
		default:
			return nil, "", apperr.Wrap(apperr.ErrGitNotInstalled, nil,
				"Git isn't installed", "install git to download from "+req.Source.Display())
		}
	}

	switch method {
	case model.MethodAPI:
		if !req.Source.GitHub() {
			return nil, "", apperr.Wrap(apperr.ErrUnsupported, nil,
				"The api method only works with github.com", "use --method sparse for other hosts")
		}
		return NewGitHubAPIDownloader(httpClient()), method, nil
	case model.MethodSparse:
		return NewSparseCheckoutDownloader(gitutil.RealRunner{}), method, nil
	}
	return nil, "", fmt.Errorf("unsupported method %q", method)
}

func httpClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{Transport: transport}
}
