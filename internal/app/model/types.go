package model

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
	RepoURL   string
	Subdir    string
	OutputDir string
	Branch    string
	Token     string
	Method    Method
	Provider  ProviderType
}

type Snapshot struct {
	Dir string
	Ref string
}

type Result struct {
	Ref    string
	Output string
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
