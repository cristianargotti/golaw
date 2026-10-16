package install

import (
	"encoding/json"
	"fmt"
	"slices"
)

// Ownership preserves the distinction between absent and preexisting values.
type Ownership struct {
	Deny       []string                   `json:"deny"`
	Original   map[string]json.RawMessage `json:"original"`
	Installed  map[string]json.RawMessage `json:"installed"`
	Missing    []string                   `json:"missing"`
	Containers []string                   `json:"containers"`
	HadDeny    bool                       `json:"had_deny"`
	Created    bool                       `json:"created"`
}

// MergeSettings retains foreign array order and the first ownership record.
func MergeSettings(data []byte, marketplace string, rules []string, prior Ownership) ([]byte, Ownership, error) {
	root, err := settingsObject(data)
	if err != nil {
		return nil, prior, err
	}
	deny, hadDeny, err := denyValues(root)
	if err != nil {
		return nil, prior, err
	}
	owned := initialOwnership(root, data, prior, hadDeny)
	deny = mergeRules(deny, rules, &owned)
	err = mergePointers(root, marketplace, &owned)
	if err != nil {
		return nil, prior, err
	}
	err = putDeny(root, deny, true)
	if err != nil {
		return nil, prior, err
	}
	encoded, err := encodeSettings(root, data)
	return encoded, owned, err
}

func initialOwnership(root map[string]json.RawMessage, data []byte, prior Ownership, hadDeny bool) Ownership {
	if prior.Original != nil {
		return prior
	}
	owned := Ownership{
		Original: map[string]json.RawMessage{},
		Created:  len(data) == 0, HadDeny: hadDeny,
	}
	for _, group := range []string{"enabledPlugins", "extraKnownMarketplaces", "permissions"} {
		if _, exists := root[group]; !exists {
			owned.Containers = append(owned.Containers, group)
		}
	}
	return owned
}

func mergeRules(deny, rules []string, owned *Ownership) []string {
	deny = removeOwned(deny, owned.Deny)
	owned.Deny = nil
	for _, rule := range rules {
		if !slices.Contains(deny, rule) {
			deny = append(deny, rule)
			owned.Deny = append(owned.Deny, rule)
		}
	}
	return deny
}

func mergePointers(root map[string]json.RawMessage, path string, owned *Ownership) error {
	market, err := marshal(map[string]any{
		"source": map[string]string{"source": "directory", "path": path},
	})
	if err != nil {
		return fmt.Errorf("encode marketplace: %w", err)
	}
	desired := map[string]json.RawMessage{
		"enabledPlugins/golaw@golaw":   json.RawMessage("true"),
		"extraKnownMarketplaces/golaw": market,
	}
	for _, key := range pointerKeys() {
		if _, exists := owned.Original[key]; !exists {
			before, err := field(root, key)
			if err != nil {
				return err
			}
			owned.Original[key] = before
			if before == nil {
				owned.Missing = append(owned.Missing, key)
			}
		}
		if err := putField(root, key, desired[key]); err != nil {
			return err
		}
	}
	owned.Installed = desired
	return nil
}

func pointerKeys() []string {
	return []string{"enabledPlugins/golaw@golaw", "extraKnownMarketplaces/golaw"}
}

func removeOwned(values, owned []string) []string {
	out := slices.Clone(values)
	for _, value := range owned {
		if index := slices.Index(out, value); index >= 0 {
			out = slices.Delete(out, index, index+1)
		}
	}
	return out
}
