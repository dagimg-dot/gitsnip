package app

import (
	"github.com/dagimg-dot/gitsnip/internal/app/downloader"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
)

func Download(opts model.DownloadOptions) error {
	dl, err := downloader.GetDownloader(opts)
	if err != nil {
		return err
	}
	return dl.Download()
}
