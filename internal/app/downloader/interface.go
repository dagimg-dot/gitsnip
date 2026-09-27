package downloader

import (
	"context"

	"github.com/dagimg-dot/gitsnip/internal/app/model"
)

type Downloader interface {
	Download(ctx context.Context, req model.Request, dir string, rep model.Reporter) (model.Snapshot, error)
}
