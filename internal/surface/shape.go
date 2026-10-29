package surface

import (
	"encoding/json"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The root manifests that make a repository a full Node or Python project.
const (
	packageManifest = "package.json"
	pythonManifest  = "pyproject.toml"
)

// The reasons OutsideShape answers, for doctor and config explain.
const (
	reasonPackage = "package.json declares a build script: a Node or TypeScript project is outside M6, which judges the files it reads"
	reasonPython  = "pyproject.toml declares a build backend: a Python package is outside M6, which judges the files it reads"
)

// OutsideShape answers the reason a repository's root shape is outside M6
// (M6.2 non-goal 1): a package.json with a build script, a pyproject.toml
// with a build backend; false when neither holds. A manifest the reader
// cannot parse claims nothing: the answer only informs, it never denies.
func OutsideShape(s model.Snapshot) (string, bool) {
	if data, err := s.Read(packageManifest); err == nil && buildScript(data) {
		return reasonPackage, true
	}
	if data, err := s.Read(pythonManifest); err == nil && buildBackend(string(data)) {
		return reasonPython, true
	}
	return "", false
}

// buildScript reports a package.json whose scripts name build.
func buildScript(data []byte) bool {
	var manifest struct {
		Scripts map[string]json.RawMessage `json:"scripts"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return false
	}
	_, found := manifest.Scripts["build"]
	return found
}

// buildBackend reports a pyproject.toml whose build-system table names a
// build-backend, as a key of the [build-system] table or as the dotted key
// build-system.build-backend at the top level. It reads the TOML line by
// line: a table header inside a multi-line string is a stated limit.
func buildBackend(text string) bool {
	table := ""
	for raw := range strings.SplitSeq(text, "\n") {
		line := strings.TrimSpace(raw)
		if hash := strings.IndexByte(line, '#'); hash >= 0 && !strings.Contains(line[:hash], "=") {
			line = strings.TrimSpace(line[:hash])
		}
		if strings.HasPrefix(line, "[") {
			table = strings.ReplaceAll(strings.Trim(line, "[]"), " ", "")
			continue
		}
		key, _, found := strings.Cut(line, "=")
		key = strings.ReplaceAll(strings.Trim(strings.TrimSpace(key), `"'`), " ", "")
		if found && (table == "build-system" && key == "build-backend" || table == "" && key == "build-system.build-backend") {
			return true
		}
	}
	return false
}
