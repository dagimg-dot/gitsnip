package downloader

import (
	"fmt"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
)

func GetDownloader(opts model.DownloadOptions) (Downloader, error) {
	switch opts.Method {
	case model.MethodTypeAPI:
		switch opts.Provider {
		case model.ProviderTypeGitHub:
			return NewGitHubAPIDownloader(opts), nil
		}
	case model.MethodTypeSparse:
		return NewSparseCheckoutDownloader(opts), nil
	}
	return nil, fmt.Errorf("unsupported provider/method")
}
