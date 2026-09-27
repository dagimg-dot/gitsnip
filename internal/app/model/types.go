package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/dagimg-dot/gitsnip/internal/pathspec"
	"github.com/dagimg-dot/gitsnip/internal/source"
)

type Method string

const (
	MethodAuto   Method = "auto"
	MethodSparse Method = "sparse"
	MethodAPI    Method = "api"
)

func ParseMethod(s string) (Method, error) {
	switch m := Method(strings.ToLower(strings.TrimSpace(s))); m {
	case "", MethodAuto:
		return MethodAuto, nil
	case MethodSparse, MethodAPI:
		return m, nil
	}
	return "", fmt.Errorf("unknown method %q, use auto, sparse or api", s)
}

type Request struct {
	Source source.Source
	Ref    string
	Paths  []pathspec.Pattern
	Output string
	Token  string
	Method Method
	Force  bool
}

type Snapshot struct {
	Dir    string
	Ref    string
	Commit string
	Paths  []pathspec.Pattern
	List   func(context.Context) ([]string, error)
}

type Result struct {
	Method Method
	Ref    string
	Commit string
	Paths  []pathspec.Pattern
	Output string
	Target string
	Files  int
	Bytes  int64
}

type Reporter interface {
	Stage(text string)
	Progress(done, total int)
	Warn(text string)
	Debug(text string)
}
