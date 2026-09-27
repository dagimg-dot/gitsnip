package pathspec

import (
	"fmt"
	"path"
	"strings"
)

type Pattern struct {
	path     string
	segments []string
	glob     bool
}

func Parse(raw string) (Pattern, error) {
	var segments []string
	for _, segment := range strings.Split(strings.ReplaceAll(strings.TrimSpace(raw), `\`, "/"), "/") {
		switch segment {
		case "", ".":
			continue
		case "..":
			return Pattern{}, fmt.Errorf("path %q can't contain \"..\"", raw)
		}
		segments = append(segments, segment)
	}

	glob := false
	for _, segment := range segments {
		if !isGlob(segment) {
			continue
		}
		if _, err := path.Match(segment, ""); err != nil {
			return Pattern{}, fmt.Errorf("path %q isn't a valid pattern", raw)
		}
		glob = true
	}

	return Pattern{path: strings.Join(segments, "/"), segments: segments, glob: glob}, nil
}

func ParseAll(raws []string) ([]Pattern, error) {
	patterns := make([]Pattern, 0, len(raws))
	for _, raw := range raws {
		p, err := Parse(raw)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, p)
	}
	return patterns, nil
}

func isGlob(segment string) bool {
	return strings.ContainsAny(segment, "*?[")
}

func (p Pattern) String() string {
	return p.path
}

func (p Pattern) IsGlob() bool {
	return p.glob
}

func (p Pattern) IsAll() bool {
	return len(p.segments) == 0
}

func (p Pattern) Rule() string {
	return "/" + p.path
}

func (p Pattern) Match(file string) bool {
	return matchSegments(p.segments, strings.Split(file, "/"))
}

func (p Pattern) IsExactly(file string) bool {
	return !p.glob && p.path == file
}

func matchSegments(pattern, file []string) bool {
	if len(pattern) == 0 {
		return true
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(file); i++ {
			if matchSegments(pattern[1:], file[i:]) {
				return true
			}
		}
		return false
	}
	if len(file) == 0 {
		return false
	}
	ok, _ := path.Match(pattern[0], file[0])
	return ok && matchSegments(pattern[1:], file[1:])
}

func (p Pattern) Anchor(isFile bool) string {
	if !p.glob {
		if isFile {
			return parent(p.path)
		}
		return p.path
	}
	var fixed []string
	for _, segment := range p.segments {
		if isGlob(segment) {
			break
		}
		fixed = append(fixed, segment)
	}
	return strings.Join(fixed, "/")
}

func parent(file string) string {
	if dir := path.Dir(file); dir != "." {
		return dir
	}
	return ""
}

func CommonDir(dirs []string) string {
	if len(dirs) == 0 {
		return ""
	}
	common := strings.Split(dirs[0], "/")
	for _, dir := range dirs[1:] {
		segments := strings.Split(dir, "/")
		n := 0
		for n < len(common) && n < len(segments) && common[n] == segments[n] {
			n++
		}
		common = common[:n]
	}
	return strings.Join(common, "/")
}
