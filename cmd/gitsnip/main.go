package main

import (
	"fmt"
	"os"

	"github.com/dagimg-dot/gitsnip/internal/cli"
	"github.com/dagimg-dot/gitsnip/internal/errors"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", errors.FormatError(err))
		os.Exit(1)
	}
}
