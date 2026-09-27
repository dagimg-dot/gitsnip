package downloader

import (
	"fmt"
	"net/http"
	"time"

	"github.com/dagimg-dot/gitsnip/internal/app/gitutil"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
)

func GetDownloader(req model.Request) (Downloader, error) {
	switch req.Method {
	case model.MethodAPI:
		if req.Provider == model.ProviderTypeGitHub {
			return NewGitHubAPIDownloader(&http.Client{Timeout: 30 * time.Second}), nil
		}
	case model.MethodSparse:
		return NewSparseCheckoutDownloader(gitutil.RealRunner{}), nil
	}
	return nil, fmt.Errorf("unsupported provider/method")
}
