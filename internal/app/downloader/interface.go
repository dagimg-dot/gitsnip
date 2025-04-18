package downloader

type Downloader interface {
	Download() error
}
