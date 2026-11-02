package install

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	version := contentVersion(files)
	for _, file := range files {
		if err := addBundleFile(plan, previous, file, version); err != nil {
			return err
		}
	}
	return nil
}

// addBundleFile places one asset of the Claude plugin, the manifest with
// the content version, and records the hook definition it validated.
func addBundleFile(plan *Plan, previous Receipt, file assets.File, version string) error {
	destination := file.Path
	switch {
	case file.Path == marketplaceManifest:
		destination = ".claude-plugin/marketplace.json"
	case !strings.HasPrefix(file.Path, "agents/claude/"):
		return nil
	}
	data, err := bundleData(file, version, MarketplaceName(plan.Receipt.Scope, plan.Receipt.ProjectID))
	if err != nil {
		return err
	}
	path := filepath.Join(plan.Receipt.Bundle, filepath.FromSlash(destination))
	if err := checkAssetOwner(path, previous); err != nil {
		return err
	}
	if err := addArtifact(plan, path, data); err != nil {
		return err
	}
	if file.Path == "agents/claude/hooks/hooks.json" {
		if err := ValidateHooks(file.Data); err != nil {
			return err
		}
		plan.Receipt.HookDefinition = file.Data
	}
	return nil
}

// bundleData is the bytes an asset is installed with: the plugin manifest
// carries the content version and the marketplace manifest the name the
// settings register.
func bundleData(file assets.File, version, name string) ([]byte, error) {
	switch file.Path {
	case pluginManifest:
		return rewriteField(file.Data, "version", func(base string) string { return base + "-" + version })
	case marketplaceManifest:
		return rewriteField(file.Data, "name", func(string) string { return name })
	}
	return file.Data, nil
}

// marketplaceManifest is the catalog the settings point the host at.
const marketplaceManifest = "agents/marketplace.json"

// pluginManifest is the Claude plugin manifest whose version the host
// caches the plugin under.
const pluginManifest = "agents/claude/.claude-plugin/plugin.json"

// contentVersion names the content of the Claude plugin: Claude Code keeps
// an installed plugin under its manifest version and never reads a new
// copy under the same one, so the version carries a digest of every other
// file of the plugin and changes whenever the skill or the hooks change.
func contentVersion(files []assets.File) string {
	var content []byte
	for _, file := range files {
		if file.Path == pluginManifest || !strings.HasPrefix(file.Path, "agents/claude/") {
			continue
		}
		content = fmt.Appendf(content, "%s\x00%d\x00", file.Path, len(file.Data))
		content = append(content, file.Data...)
	}
	sum := sha256.Sum256(content)
	return "g" + hex.EncodeToString(sum[:6])
}

// rewriteField sets one string field of a JSON manifest from its current
// value, keeping every other field as written.
func rewriteField(data []byte, key string, value func(string) string) ([]byte, error) {
	var manifest map[string]json.RawMessage
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("%w: read a plugin manifest: %w", ErrHooks, err)
	}
	var current string
	if err := json.Unmarshal(manifest[key], &current); err != nil || current == "" {
		return nil, fmt.Errorf("%w: a plugin manifest carries no %s", ErrHooks, key)
	}
	encoded, err := json.Marshal(value(current))
	if err != nil {
		return nil, fmt.Errorf("encode the manifest %s: %w", key, err)
	}
	manifest[key] = encoded
	out, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode a plugin manifest: %w", err)
	}
	return append(out, '\n'), nil
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
