package downloader

import (
	"fmt"

	"github.com/dagimg-dot/gitsnip/internal/app/model"
)

type Downloader interface {
	Download() error
}

func NewSparseCheckoutDownloader(opts model.DownloadOptions) Downloader {
	return &sparseCheckoutDownloader{opts: opts}
}

type sparseCheckoutDownloader struct {
	opts model.DownloadOptions
}

func (s *sparseCheckoutDownloader) Download() error {
	return fmt.Errorf("sparse-checkout method not implemented yet")
}
