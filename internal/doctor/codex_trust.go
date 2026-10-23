package doctor

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/cristianargotti/golaw/internal/install"
)

// The sources of the Codex trust and the status of one handler, as Codex
// names them (survey section 6.1), plus absent for a handler the hooks
// file no longer holds.
const (
	sourceRecord     = "record"
	statusTrusted    = "trusted"
	statusModified   = "modified"
	statusUntrusted  = "untrusted"
	statusManaged    = "managed"
	statusAbsent     = "absent"
	statusDisabled   = "disabled"
	projectTrusted   = "trusted"
	projectAncestor  = "ancestor"
	projectUntrusted = "untrusted"
	featureOff       = "off"
	codexConfigName  = "config.toml"
)

// trustFact is what decides whether Codex runs the installed handlers: the
// status of each, the hooks feature, and with --scope project the trust of
// the project layer; Source says where it was read, and Reason why Codex
// itself was not asked.
type trustFact struct {
	Source  string      `json:"source"`
	Reason  string      `json:"reason"`
	Config  string      `json:"config"`
	Feature string      `json:"feature"`
	Project string      `json:"project"`
	Hooks   []hookTrust `json:"hooks"`
	Error   string      `json:"error"`
}

// hookTrust is the status of one handler under the key Codex keys it by.
type hookTrust struct {
	Key    string `json:"key"`
	Status string `json:"status"`
}

// codexConfig is the part of the Codex user configuration beside the trust
// record that decides whether a hook runs, and the bytes Codex reads of the
// AGENTS.md chain.
type codexConfig struct {
	Features struct {
		Hooks *bool `toml:"hooks"`
	} `toml:"features"`
	Projects map[string]struct {
		TrustLevel string `toml:"trust_level"`
	} `toml:"projects"`
	Hooks struct {
		State map[string]struct {
			Enabled *bool `toml:"enabled"`
		} `toml:"state"`
	} `toml:"hooks"`
	ProjectDocMaxBytes *int64 `toml:"project_doc_max_bytes"`
}

// codexRecordTrust reads the trust of every installed handler from the
// Codex record (A20, D14): the hash of the handler as the hooks file holds
// it now, at the position it holds now, against the trusted hash of that
// key in <codex home>/config.toml. Doctor never writes the record.
func codexRecordTrust(input Input) trustFact {
	receipt, fact := readReceiptFact(input.ReceiptPath)
	if fact.State != receiptRead {
		return trustFact{}
	}
	trust := trustFact{Source: sourceRecord, Config: filepath.Join(input.ConfigDir, codexConfigName)}
	config, err := readCodexConfig(trust.Config)
	if err != nil {
		trust.Error = err.Error()
		return trust
	}
	trust.Feature, trust.Project = config.feature(), config.project(input)
	trusted, err := install.CodexTrusted(trust.Config)
	if err != nil {
		trust.Error = err.Error()
		return trust
	}
	slots, err := readSlots(receipt.HooksPath, false)
	if err != nil {
		slots = nil
	}
	for _, entry := range receipt.Entries {
		status := recordStatus(receipt.HooksPath, entry, slots, trusted, runtime.GOOS)
		if config.disabled(status.Key) {
			status.Status = statusDisabled
		}
		trust.Hooks = append(trust.Hooks, status)
	}
	return trust
}

// disabled reports whether the configuration turns a handler off under its
// key, enabled = false in hooks.state, which Codex never runs whatever its
// trust (survey section 6.2).
func (c codexConfig) disabled(key string) bool {
	state, found := c.Hooks.State[key]
	return found && state.Enabled != nil && !*state.Enabled
}

// readCodexConfig reads the Codex user configuration; an absent file is
// the defaults.
func readCodexConfig(path string) (codexConfig, error) {
	var config codexConfig
	_, err := toml.DecodeFile(path, &config)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return codexConfig{}, fmt.Errorf("read the Codex configuration %s: %w", path, err)
	}
	return config, nil
}

// feature is off only when the configuration turns the hooks feature off.
func (c codexConfig) feature() string {
	if c.Features.Hooks != nil && !*c.Features.Hooks {
		return featureOff
	}
	return ""
}

// project is the trust of the project layer the scope needs: the root
// itself trusted, only an ancestor of it, or neither.
func (c codexConfig) project(input Input) string {
	if input.Scope != scopeProject {
		return ""
	}
	root := filepath.Clean(input.Root)
	out := projectUntrusted
	for path, entry := range c.Projects {
		if entry.TrustLevel != projectTrusted {
			continue
		}
		dir := filepath.Clean(path)
		switch {
		case dir == root:
			return projectTrusted
		case strings.HasPrefix(root, dir+string(filepath.Separator)):
			out = projectAncestor
		}
	}
	return out
}

// recordStatus is the status of one handler against the record: the
// handler found by its definition, or the changed one at the position of
// the receipt, hashed as Codex hashes it.
func recordStatus(hooksPath string, entry install.Entry, slots map[string][]slot, trusted map[string]string, goos string) hookTrust {
	current, found := handlerSlot(slots[entry.Event], entry)
	if !found {
		return hookTrust{Key: install.CodexTrustKey(hooksPath, entry.Event, entry.Group, entry.Handler), Status: statusAbsent}
	}
	key := install.CodexTrustKey(hooksPath, entry.Event, current.Group, current.Handler)
	hash, err := install.CodexTrustHash(entry.Event, current.Matcher, current.Raw, goos)
	recorded, known := trusted[key]
	switch {
	case err != nil || !known:
		return hookTrust{Key: key, Status: statusUntrusted}
	case recorded != hash:
		return hookTrust{Key: key, Status: statusModified}
	default:
		return hookTrust{Key: key, Status: statusTrusted}
	}
}

// handlerSlot is the slot holding the installed definition, or the slot at
// the position the receipt recorded, which holds a changed handler.
func handlerSlot(slots []slot, entry install.Entry) (slot, bool) {
	if index := locate(slots, entry.Definition); index >= 0 {
		return slots[index], true
	}
	for _, candidate := range slots {
		if candidate.Group == entry.Group && candidate.Handler == entry.Handler {
			return candidate, true
		}
	}
	return slot{}, false
}
