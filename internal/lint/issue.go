package lint

import (
	"path/filepath"
	"strings"
)

// severityError is the word of a tool that reports no level of its own,
// and the error level of a tool that counts its levels.
const severityError = "error"

// Issue is one finding a tool printed, before the rule map judges it.
type Issue struct {
	Tool string
	// Rule is the rule as the tool names it: "sonarjs/no-nested-conditional",
	// "S307", "DL3002", "SC2086", "CKV_AWS_18".
	Rule string
	// Path is slash separated and relative to the root, empty when the tool
	// read stdin, so the hook fills in the path it wrote.
	Path    string
	Line    int
	Column  int
	Message string
	// Severity is the tool's own word, kept for the report only.
	Severity string
	// Suppressed is true for a finding the tool reports as suppressed by a
	// comment in the source, which M6.7 check 5 allows when it names its
	// rules and gives a reason, so golaw lint reports it as a note.
	Suppressed bool
}

// relative answers file as a slash path inside root. Both sides are read in
// slash form whatever the OS, so a Windows path a tool printed reads the
// same on every runner; a drive letter root compares without case, as the
// file system does.
func relative(root, file string) string {
	switch file {
	case "", "-", "/dev/stdin":
		return ""
	}
	f := toSlash(file)
	if root == "" {
		return f
	}
	prefix := strings.TrimSuffix(toSlash(root), "/") + "/"
	if len(f) <= len(prefix) {
		return f
	}
	head := f[:len(prefix)]
	if head == prefix || (hasDrive(prefix) && strings.EqualFold(head, prefix)) {
		return f[len(prefix):]
	}
	return f
}

func toSlash(path string) string {
	return strings.ReplaceAll(filepath.ToSlash(path), `\`, "/")
}

func hasDrive(path string) bool {
	return len(path) >= 2 && path[1] == ':' &&
		((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z'))
}
