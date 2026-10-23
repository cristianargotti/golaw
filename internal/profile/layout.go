package profile

import (
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/policy"
)

// The layout of a bundle and of the config directory that holds it (D2, D3).
const (
	// RulesFile is the data file beside policy.StampFile.
	RulesFile = "rules.yml"
	// SchemaVersion is the golaw_profile key every rules.yml carries.
	SchemaVersion = 1
	// DigestPrefix starts every bundle digest.
	DigestPrefix = "sha256:"
)

// Dir is the current bundle of a name under the config directory, the
// directory of policy.StampPath.
func Dir(configDir, name string) string {
	return filepath.Dir(policy.StampPath(configDir, name))
}
