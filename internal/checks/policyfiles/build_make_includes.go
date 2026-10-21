package policyfiles

import (
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

func (r *makeReader) include(line makeLine, condition string) (bool, error) {
	words := makeWords(line.text)
	if len(words) == 0 {
		return false, nil
	}
	optional := false
	switch words[0] {
	case "include":
	case "-include", "sinclude":
		optional = true
	default:
		return false, nil
	}
	for _, name := range words[1:] {
		if err := r.includeName(name, line, optional, condition); err != nil {
			return true, err
		}
	}
	return true, nil
}

func (r *makeReader) includeName(name string, line makeLine, optional bool, condition string) error {
	if strings.Contains(name, "$") {
		return outside(line.file, line.line, name)
	}
	paths, err := makeIncludePaths(r.in, name)
	if err != nil {
		return err
	}
	found := false
	for _, file := range paths {
		if r.active[file] {
			return outside(line.file, line.line, "include cycle "+name)
		}
		present, loadErr := r.load(file, condition)
		if loadErr != nil {
			return loadErr
		}
		found = found || present
	}
	if !found {
		r.result.missing = append(r.result.missing, makeInclude{
			name: name, optional: optional, file: line.file, line: line.line, condition: condition,
		})
	}
	return nil
}

func makeIncludePaths(in model.Input, name string) ([]string, error) {
	pattern, err := makeIncludePath(name)
	if err != nil {
		return nil, err
	}
	if _, err := path.Match(pattern, ""); err != nil {
		return nil, parseError(name, err)
	}
	if !strings.ContainsAny(pattern, "*?[") {
		return []string{pattern}, nil
	}
	files := model.Select(in.Snapshot, func(file string) bool {
		matched, matchErr := path.Match(pattern, file)
		return matchErr == nil && matched
	})
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	slices.Sort(paths)
	return paths, nil
}

func makeIncludePath(name string) (string, error) {
	slashed := strings.ReplaceAll(name, `\`, "/")
	cleaned := path.Clean(slashed)
	if path.IsAbs(slashed) || cleaned == "." || cleaned == ".." ||
		strings.HasPrefix(cleaned, "../") || makeDriveQualified(slashed) {
		return "", parseError(name, model.ErrUnreadable)
	}
	return path.Clean(name), nil
}

func makeDriveQualified(name string) bool {
	return len(name) > 1 && name[1] == ':' &&
		(name[0] >= 'a' && name[0] <= 'z' || name[0] >= 'A' && name[0] <= 'Z')
}
