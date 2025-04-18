package main

import (
	"github.com/dagimg-dot/gitsnip/internal/cli"
	"os"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
