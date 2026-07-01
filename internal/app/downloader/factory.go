package downloader

import (
	"fmt"
	"net/http"
	"time"

	"github.com/dagimg-dot/gitsnip/internal/app/gitutil"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
)

func GetDownloader(opts model.DownloadOptions) (Downloader, error) {
	switch opts.Method {
	case model.MethodTypeAPI:
		switch opts.Provider {
		case model.ProviderTypeGitHub:
			client := &http.Client{Timeout: 30 * time.Second}
			return NewGitHubAPIDownloader(opts, client), nil
		}
	case model.MethodTypeSparse:
		return NewSparseCheckoutDownloader(opts, gitutil.RealRunner{}), nil
	}
	return nil, fmt.Errorf("unsupported provider/method")
}
