package install

import (
	"encoding/json"
	"fmt"
	"os"
)

// Artifact permits doctor and uninstall to compare the exact installed bytes.
type Artifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Receipt preserves the first ownership record through repeated installations.
type Receipt struct {
	Schema         int             `json:"schema"`
	ProjectID      string          `json:"project_id"`
	Scope          string          `json:"scope"`
	Version        string          `json:"version"`
	SettingsPath   string          `json:"settings_path"`
	Bundle         string          `json:"bundle"`
	Artifacts      []Artifact      `json:"artifacts"`
	HookDefinition json.RawMessage `json:"hook_definition"`
	Ownership      Ownership       `json:"ownership"`
}

// ReadReceipt refuses corrupted ownership evidence instead of guessing.
func ReadReceipt(path string) (Receipt, error) {
	data, err := os.ReadFile(path) //nolint:gosec // The caller selects the state receipt.
	if err != nil {
		return Receipt{}, fmt.Errorf("read receipt: %w", err)
	}
	var receipt Receipt
	if err := strictJSON(data, &receipt); err != nil {
		return Receipt{}, err
	}
	if receipt.Schema != 1 || receipt.ProjectID == "" || receipt.Bundle == "" {
		return Receipt{}, fmt.Errorf("receipt identity: %w", ErrSettings)
	}
	return receipt, nil
}
