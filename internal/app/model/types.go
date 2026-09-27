package model

import (
	"context"

	"github.com/dagimg-dot/gitsnip/internal/pathspec"
	"github.com/dagimg-dot/gitsnip/internal/source"
)

type Method string

const (
	MethodSparse Method = "sparse"
	MethodAPI    Method = "api"
)

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
	Ref    string
	Commit string
	Paths  []pathspec.Pattern
	Output string
	Files  int
	Bytes  int64
}

type Reporter interface {
	Stage(text string)
	Progress(done, total int)
	Warn(text string)
	Debug(text string)
}

type Discard struct{}

func (Discard) Stage(string)      {}
func (Discard) Progress(int, int) {}
func (Discard) Warn(string)       {}
func (Discard) Debug(string)      {}
