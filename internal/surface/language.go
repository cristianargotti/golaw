package surface

import (
	"path"
	"strings"
)

// Language is a surface language of M6.3.1, named as golaw.yml and the
// tool table name it.
type Language string

// The surface languages, in the order Languages answers them.
const (
	JavaScript     Language = "javascript"
	TypeScript     Language = "typescript"
	JSX            Language = "jsx"
	TSX            Language = "tsx"
	HTML           Language = "html"
	CSS            Language = "css"
	Python         Language = "python"
	Dockerfile     Language = "dockerfile"
	CloudFormation Language = "cloudformation"
	Shell          Language = "shell"
	YAML           Language = "yaml"
)

// Languages answers every surface language in report order.
func Languages() []Language {
	return []Language{JavaScript, TypeScript, JSX, TSX, HTML, CSS, Python, Dockerfile, CloudFormation, Shell, YAML}
}

// byExtension is the language an extension alone settles (M6.3.1).
func byExtension(ext string) Language {
	switch ext {
	case ".js", ".mjs", ".cjs":
		return JavaScript
	case ".ts", ".mts", ".cts":
		return TypeScript
	case ".jsx":
		return JSX
	case ".tsx":
		return TSX
	case ".html", ".htm":
		return HTML
	case ".css":
		return CSS
	case ".py":
		return Python
	case ".sh", ".bash":
		return Shell
	}
	return ""
}

// contentDecides reports the extensions whose bytes settle the language: a
// shebang for a file without one, the CloudFormation markers for the data
// files.
func contentDecides(ext string) bool {
	switch ext {
	case "", ".yml", ".yaml", ".json", ".template":
		return true
	}
	return false
}

// ByName answers the language the name of a slash path settles, with true,
// or false when the content decides. A name the rules settle as no surface
// language answers the empty language with true.
func ByName(name string) (Language, bool) {
	base := path.Base(name)
	if dockerfileName(base) {
		return Dockerfile, true
	}
	ext := strings.ToLower(path.Ext(base))
	if contentDecides(ext) {
		return "", false
	}
	return byExtension(ext), true
}

// dockerfileName reports the names of M6.3.1. A Dockerfile.dockerignore is
// the ignore file of that Dockerfile and a Dockerfile.tmpl or .tpl a
// template whose rendering is one, so neither is a build file to judge.
func dockerfileName(base string) bool {
	switch {
	case base == "Dockerfile", base == "Containerfile":
		return true
	case strings.HasSuffix(base, ".dockerignore"), strings.HasSuffix(base, ".tmpl"), strings.HasSuffix(base, ".tpl"):
		return false
	}
	return strings.HasPrefix(base, "Dockerfile.") || strings.HasSuffix(base, ".Dockerfile")
}
