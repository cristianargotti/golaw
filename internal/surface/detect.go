package surface

import (
	"bytes"
	"path"
	"strings"
)

// FileInfo is what detection knows of one file.
type FileInfo struct {
	Path     string
	Language Language // empty when the file is no surface language
	Class    Class
	Regions  []Region // html only
}

// Detect answers the language, the class and, for HTML, the regions of a
// file from its slash path and its bytes (M6.3.1). A YAML or JSON file
// that carries the CloudFormation markers and does not parse answers
// ErrCloudFormation; a .yml or .yaml file keeps the yaml language then, so
// the YAML linter still reads it.
func Detect(name string, content []byte) (FileInfo, error) {
	language, settled := ByName(name)
	var err error
	if !settled {
		language, err = byContent(name, content)
	}
	info := FileInfo{Path: name, Language: language}
	if language == "" {
		return info, err
	}
	info.Class = classOf(name, language, content)
	if language == HTML {
		info.Regions = Regions(content)
	}
	return info, err
}

// byContent answers the language the bytes settle for a name ByName left
// open: the shebang of a file without an extension, the template markers
// of a data file.
func byContent(name string, content []byte) (Language, error) {
	ext := strings.ToLower(path.Ext(path.Base(name)))
	if ext == "" {
		return byShebang(content), nil
	}
	found, err := template(content)
	switch {
	case found:
		return CloudFormation, nil
	case yamlExt(ext):
		return YAML, err
	}
	return "", err
}

// yamlExt reports a lower case extension of a YAML document, which stays
// yaml when it is no template.
func yamlExt(ext string) bool {
	return ext == ".yml" || ext == ".yaml"
}

// byShebang answers the language a #! line names: node, python with or
// without a version, sh, bash, dash or zsh, through env and its flags too.
func byShebang(content []byte) Language {
	line, _, _ := bytes.Cut(content, []byte("\n"))
	rest, found := bytes.CutPrefix(bytes.TrimRight(line, "\r"), []byte("#!"))
	if !found {
		return ""
	}
	fields := strings.Fields(string(rest))
	if len(fields) > 0 && path.Base(fields[0]) == "env" {
		fields = fields[1:]
		for len(fields) > 0 && (strings.HasPrefix(fields[0], "-") || strings.Contains(fields[0], "=")) {
			fields = fields[1:]
		}
	}
	if len(fields) == 0 {
		return ""
	}
	interpreter := path.Base(fields[0])
	switch {
	case interpreter == "node":
		return JavaScript
	case interpreter == "sh", interpreter == "bash", interpreter == "dash", interpreter == "zsh":
		return Shell
	case strings.HasPrefix(interpreter, "python") && strings.Trim(interpreter[len("python"):], "0123456789.") == "":
		return Python
	}
	return ""
}
