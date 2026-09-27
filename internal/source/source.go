package source

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

type Source struct {
	Host    string
	Owner   string
	Repo    string
	URL     string
	Ref     string
	RefPath string
	Path    string
}

var (
	scpLike = regexp.MustCompile(`^([A-Za-z0-9._-]+)@([A-Za-z0-9.-]+):(.+)$`)
	segment = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)
)

func Parse(raw string) (Source, error) {
	s := strings.TrimSpace(raw)
	var src Source
	var err error
	switch first, _, _ := strings.Cut(s, "/"); {
	case s == "":
		return Source{}, errors.New("the source is empty")
	case strings.HasPrefix(s, "file://"):
		src, err = parseFile(s)
	case !strings.Contains(s, "://") && scpLike.MatchString(s):
		src, err = parseSCP(s)
	case strings.Contains(s, "://"):
		src, err = parseURL(s)
	case strings.ContainsAny(first, ".:"):
		body, ref := trailingRef(s)
		if src, err = parseURL("https://" + body); err == nil {
			err = src.applyRef(ref)
		}
	default:
		body, ref := trailingRef(s)
		if src, err = parseShorthand(body); err == nil {
			err = src.applyRef(ref)
		}
	}
	if err != nil {
		return Source{}, err
	}
	if src.Ref != "" && src.RefPath != "" {
		return Source{}, fmt.Errorf("%q names a branch twice", raw)
	}
	return src, nil
}

func (s *Source) GitHub() bool {
	return s.Host == "github.com"
}

func (s *Source) Display() string {
	switch {
	case s.GitHub():
		return s.Owner + "/" + s.Repo
	case s.Host != "":
		return s.Host + "/" + s.Owner + "/" + s.Repo
	default:
		return s.Repo
	}
}

// trailingRef splits "owner/repo/path@ref". An @ right after a slash belongs to
// the path, as in node_modules/@types.
func trailingRef(s string) (body, ref string) {
	if i := strings.LastIndex(s, "@"); i > 0 && i < len(s)-1 && s[i-1] != '/' {
		return s[:i], s[i+1:]
	}
	return s, ""
}

func (s *Source) applyRef(ref string) error {
	if ref == "" {
		return nil
	}
	if s.Ref != "" || s.RefPath != "" {
		return errors.New("the source names a branch twice")
	}
	s.Ref = ref
	return nil
}

func parseShorthand(s string) (Source, error) {
	segs := split(s)
	if len(segs) < 2 {
		return Source{}, fmt.Errorf("%q isn't owner/repo, a link or a git remote", s)
	}
	src := Source{Host: "github.com", Owner: segs[0], Repo: trimGit(segs[1]), Path: strings.Join(segs[2:], "/")}
	if err := validate(s, []string{src.Owner, src.Repo}); err != nil {
		return Source{}, err
	}
	src.URL = githubURL(&src)
	return src, nil
}

func parseSCP(s string) (Source, error) {
	m := scpLike.FindStringSubmatch(s)
	user, host, repoPath := m[1], strings.ToLower(m[2]), m[3]
	segs := split(repoPath)
	if len(segs) < 2 {
		return Source{}, fmt.Errorf("%q doesn't name an owner and a repository", s)
	}
	last, ref := splitRef(segs[len(segs)-1])
	segs[len(segs)-1] = last
	if err := validate(s, segs); err != nil {
		return Source{}, err
	}
	return Source{
		Host:  host,
		Owner: strings.Join(segs[:len(segs)-1], "/"),
		Repo:  trimGit(last),
		Ref:   ref,
		URL:   user + "@" + host + ":" + strings.Join(segs, "/"),
	}, nil
}

func parseFile(s string) (Source, error) {
	u, err := url.Parse(s)
	if err != nil || u.Path == "" {
		return Source{}, fmt.Errorf("%q isn't a valid file URL", s)
	}
	name, ref := splitRef(path.Base(u.Path))
	u.Path = path.Join(path.Dir(u.Path), name)
	u.RawPath = ""
	return Source{Repo: trimGit(name), Ref: ref, URL: u.String()}, nil
}

func parseURL(s string) (Source, error) {
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return Source{}, fmt.Errorf("%q isn't a valid URL", s)
	}
	switch u.Scheme {
	case "http", "https":
		return parseWeb(s, u)
	case "ssh", "git", "git+ssh":
		return parseRemote(s, u)
	}
	return Source{}, fmt.Errorf("%s:// links aren't supported", u.Scheme)
}

func parseRemote(s string, u *url.URL) (Source, error) {
	segs := split(u.Path)
	if len(segs) < 2 {
		return Source{}, fmt.Errorf("%q doesn't name an owner and a repository", s)
	}
	last, ref := splitRef(segs[len(segs)-1])
	segs[len(segs)-1] = last
	if err := validate(s, segs); err != nil {
		return Source{}, err
	}
	clone := *u
	clone.Path, clone.RawPath = "/"+strings.Join(segs, "/"), ""
	return Source{
		Host:  strings.ToLower(u.Hostname()),
		Owner: strings.Join(segs[:len(segs)-1], "/"),
		Repo:  trimGit(last),
		Ref:   ref,
		URL:   clone.String(),
	}, nil
}

func parseWeb(s string, u *url.URL) (Source, error) {
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	repoSegs, rest := splitRepoPath(host, split(u.Path))
	if len(repoSegs) < 2 {
		return Source{}, fmt.Errorf("%q doesn't name an owner and a repository", s)
	}
	last, ref := splitRef(repoSegs[len(repoSegs)-1])
	repoSegs[len(repoSegs)-1] = last
	if err := validate(s, repoSegs); err != nil {
		return Source{}, err
	}

	src := Source{Host: host, Owner: strings.Join(repoSegs[:len(repoSegs)-1], "/"), Repo: trimGit(last), Ref: ref}
	if err := src.locate(s, rest); err != nil {
		return Source{}, err
	}

	if src.GitHub() {
		src.URL = githubURL(&src)
	} else {
		clone := url.URL{Scheme: u.Scheme, User: u.User, Host: u.Host, Path: "/" + strings.Join(repoSegs, "/")}
		src.URL = clone.String()
	}
	return src, nil
}

func splitRepoPath(host string, segs []string) (repo, rest []string) {
	for i, seg := range segs {
		if seg == "-" {
			return segs[:i], segs[i+1:]
		}
	}
	if len(segs) <= 2 || host == "github.com" || host == "bitbucket.org" || host == "git.sr.ht" {
		return segs[:min(2, len(segs))], segs[min(2, len(segs)):]
	}
	for i := 2; i < len(segs); i++ {
		switch segs[i] {
		case "tree", "blob", "src", "raw":
			return segs[:i], segs[i:]
		}
	}
	return segs, nil
}

func (s *Source) locate(raw string, rest []string) error {
	if len(rest) == 0 {
		return nil
	}
	rel := func(from int) string {
		if from >= len(rest) {
			return ""
		}
		return strings.Join(rest[from:], "/")
	}

	switch {
	case s.Host == "git.sr.ht" && rest[0] == "tree":
		for i := 2; i < len(rest); i++ {
			if rest[i] == "item" {
				s.Ref, s.Path = strings.Join(rest[1:i], "/"), rel(i+1)
				return nil
			}
		}
		s.Ref = rel(1)
	case rest[0] == "tree" || rest[0] == "blob" || rest[0] == "raw":
		s.RefPath = rel(1)
	case rest[0] == "src" && len(rest) > 1 && (rest[1] == "branch" || rest[1] == "tag" || rest[1] == "commit"):
		s.RefPath = rel(2)
	case rest[0] == "src" && s.Host == "bitbucket.org":
		s.RefPath = rel(1)
	case rest[0] == "commit" && len(rest) > 1:
		s.Ref = rest[1]
	case rest[0] == "releases" && len(rest) > 2 && rest[1] == "tag":
		s.Ref = rel(2)
	case s.GitHub():
		s.Path = rel(0)
	default:
		return fmt.Errorf("can't tell which part of %q is the repository", raw)
	}
	return nil
}

func githubURL(s *Source) string {
	return "https://github.com/" + s.Owner + "/" + s.Repo + ".git"
}

func split(p string) []string {
	var segs []string
	for _, seg := range strings.Split(p, "/") {
		if seg != "" {
			segs = append(segs, seg)
		}
	}
	return segs
}

func splitRef(seg string) (name, ref string) {
	if i := strings.LastIndex(seg, "@"); i > 0 {
		return seg[:i], seg[i+1:]
	}
	return seg, ""
}

func trimGit(name string) string {
	return strings.TrimSuffix(name, ".git")
}

func validate(raw string, segs []string) error {
	for _, seg := range segs {
		if !segment.MatchString(seg) || seg == "." || seg == ".." {
			return fmt.Errorf("%q isn't a valid repository name", raw)
		}
	}
	return nil
}
