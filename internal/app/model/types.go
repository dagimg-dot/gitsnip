package model

import "github.com/dagimg-dot/gitsnip/internal/pathspec"

type Method string

const (
	MethodSparse Method = "sparse"
	MethodAPI    Method = "api"
)

type ProviderType string

const (
	ProviderTypeGitHub ProviderType = "github"
)

type Request struct {
	RepoURL  string
	Ref      string
	Paths    []pathspec.Pattern
	Output   string
	Token    string
	Method   Method
	Provider ProviderType
}

type Snapshot struct {
	Dir    string
	Ref    string
	Commit string
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
