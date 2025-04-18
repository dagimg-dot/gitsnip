package model

type MethodType string

const (
	MethodTypeSparse MethodType = "sparse"
	MethodTypeAPI    MethodType = "api"
)

type ProviderType string

const (
	ProviderTypeGitHub ProviderType = "github"
)

type DownloadOptions struct {
	RepoURL   string
	Subdir    string
	OutputDir string
	Branch    string
	Token     string
	Method    MethodType
	Provider  ProviderType
	Quiet     bool
}
