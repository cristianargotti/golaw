package bootstrap

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// Suggestion keeps the evidence separate from a decision to enable a capability.
type Suggestion struct {
	Capability, Evidence string
}

// Suggest respects the capability floor so its printed command is one init accepts.
func Suggest(root string, enabled []string) ([]Suggestion, error) {
	root = filepath.Clean(root)
	evidence, err := suggestEvidence(root)
	if err != nil {
		return nil, err
	}
	if suggestDirectory(root, "internal/domain") && suggestDirectory(root, "internal/application") {
		evidence["hexagonal"] = "internal/domain"
	}
	var suggestions []Suggestion
	for _, capability := range model.Capabilities() {
		if rel := evidence[capability]; rel != "" && !slices.Contains(enabled, capability) {
			if policy.CheckCapabilities(append(slices.Clone(enabled), capability)) != nil {
				continue
			}
			suggestions = append(suggestions, Suggestion{Capability: capability, Evidence: rel})
		}
	}
	return suggestions, nil
}

func suggestEvidence(root string) (map[string]string, error) {
	evidence := make(map[string]string)
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %q: %w", name, walkErr)
		}
		if entry.IsDir() && name != root && excludedSuggestionDirectory(entry.Name()) {
			return fs.SkipDir
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return fmt.Errorf("relative evidence path %q: %w", name, err)
		}
		rel = filepath.ToSlash(rel)
		if capability := fileEvidence(rel); capability != "" && evidence[capability] == "" {
			evidence[capability] = rel
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("bootstrap: suggest capabilities: %w", err)
	}
	return evidence, nil
}

func excludedSuggestionDirectory(name string) bool {
	return name == "testdata" || name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".")
}

func suggestDirectory(root, rel string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && info.IsDir()
}

func fileEvidence(rel string) string {
	name := path.Base(rel)
	if name == "Dockerfile" ||
		(strings.HasPrefix(name, "Dockerfile.") && name != "Dockerfile.") ||
		(strings.HasSuffix(name, ".Dockerfile") && name != ".Dockerfile") {
		return "image"
	}
	extension := path.Ext(name)
	if extension == ".yaml" || extension == ".yml" || extension == ".json" {
		switch strings.TrimSuffix(name, extension) {
		case "openapi", "swagger":
			return "api"
		case "sqlc":
			return "db"
		}
	}
	if extension == ".sql" && slices.Contains(strings.Split(path.Dir(rel), "/"), "migrations") {
		return "db"
	}
	return ""
}
