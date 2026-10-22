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
// Agent, HooksPath, HooksCreated, Entries and Blocks belong to the agents
// whose hooks live in a hooks file the person shares with other tools;
// HooksCreated says the install created that file. A receipt without
// Agent is a Claude receipt, as every receipt before them was.
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
	Agent          string          `json:"agent,omitempty"`
	HooksPath      string          `json:"hooks_path,omitempty"`
	HooksCreated   bool            `json:"hooks_created,omitempty"`
	Entries        []Entry         `json:"entries,omitempty"`
	Blocks         []Block         `json:"blocks,omitempty"`
}

// Entry is one handler an installation merged into a hooks file, at the
// position a host keys its trust by: the event, the index of its group in
// the list of the event, the index of the handler in its group, the
// matcher of the group and the exact JSON of the handler as written.
type Entry struct {
	Event      string          `json:"event"`
	Group      int             `json:"group"`
	Handler    int             `json:"handler"`
	Matcher    string          `json:"matcher,omitempty"`
	Definition json.RawMessage `json:"definition"`
}

// Block is a marked block an installation wrote into a text file the
// person owns: the file, the hash of the block from its begin marker to
// its end marker, and whether the installation created the file, which
// uninstall then removes once the block is its only content.
type Block struct {
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Created bool   `json:"created"`
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
