package main

import (
	"fmt"
	"os"

	"github.com/dagimg-dot/gitsnip/internal/apperr"
	"github.com/dagimg-dot/gitsnip/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", apperr.FormatError(err))
		os.Exit(1)
	}
}
