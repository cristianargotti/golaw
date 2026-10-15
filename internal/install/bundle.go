package install

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/assets"
)

func addBundle(plan *Plan, previous Receipt) error {
	files, err := assets.Files()
	if err != nil {
		return fmt.Errorf("load plugin assets: %w", err)
	}
	for _, file := range files {
		destination := file.Path
		switch {
		case file.Path == "agents/marketplace.json":
			destination = ".claude-plugin/marketplace.json"
		case !strings.HasPrefix(file.Path, "agents/claude/"):
			continue
		}
		path := filepath.Join(plan.Receipt.Bundle, filepath.FromSlash(destination))
		if err := checkAssetOwner(path, previous); err != nil {
			return err
		}
		if err := addArtifact(plan, path, file.Data); err != nil {
			return err
		}
		if file.Path == "agents/claude/hooks/hooks.json" {
			if err := ValidateHooks(file.Data); err != nil {
				return err
			}
			plan.Receipt.HookDefinition = file.Data
		}
	}
	return nil
}

func checkAssetOwner(path string, previous Receipt) error {
	data, exists, err := readOptional(path)
	if err != nil || !exists {
		return err
	}
	index := slices.IndexFunc(previous.Artifacts, func(artifact Artifact) bool {
		return artifact.Path == path
	})
	if index < 0 || previous.Artifacts[index].SHA256 != Digest(data) {
		return fmt.Errorf("plugin asset %s: %w", path, ErrChanged)
	}
	return nil
}

func addArtifact(plan *Plan, path string, data []byte) error {
	if err := addChange(plan, path, data, false); err != nil {
		return err
	}
	plan.Receipt.Artifacts = append(plan.Receipt.Artifacts, Artifact{
		Path: path, SHA256: Digest(data),
	})
	return nil
}
