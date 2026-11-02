package install

import (
	"encoding/json"
	"fmt"
	"maps"
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
// name is the marketplace the settings point at, MarketplaceName of the
// installation.
func MergeSettings(data []byte, name, marketplace string, rules []string, prior Ownership) ([]byte, Ownership, error) {
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
	err = mergePointers(root, name, marketplace, &owned)
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

func mergePointers(root map[string]json.RawMessage, name, path string, owned *Ownership) error {
	market, err := marshal(map[string]any{
		"source": map[string]string{"source": "directory", "path": path},
	})
	if err != nil {
		return fmt.Errorf("encode marketplace: %w", err)
	}
	desired := map[string]json.RawMessage{
		"enabledPlugins/golaw@" + name:   json.RawMessage("true"),
		"extraKnownMarketplaces/" + name: market,
	}
	if name != userMarketplace {
		// The host keeps one path per marketplace name across every scope,
		// so a project marketplace under its own name turns off the person's
		// plugin in this project and never resolves to that older copy.
		desired["enabledPlugins/golaw@"+userMarketplace] = json.RawMessage("false")
	}
	if err := dropStale(root, desired, owned); err != nil {
		return err
	}
	for _, key := range slices.Sorted(maps.Keys(desired)) {
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

// dropStale gives back the original value of every field an earlier
// installation wrote that this one no longer writes, such as the shared
// marketplace name of a project installed before it had its own.
func dropStale(root, desired map[string]json.RawMessage, owned *Ownership) error {
	for _, key := range pointerKeys(*owned) {
		if _, kept := desired[key]; kept {
			continue
		}
		original := owned.Original[key]
		if slices.Contains(owned.Missing, key) {
			original = nil
		}
		if err := putField(root, key, original); err != nil {
			return err
		}
		delete(owned.Original, key)
		owned.Missing = slices.DeleteFunc(owned.Missing, func(missing string) bool { return missing == key })
	}
	return nil
}

// userMarketplace is the marketplace name of a user installation.
const userMarketplace = "golaw"

// MarketplaceName is the marketplace an installation registers: golaw for
// the person, and for a project a name of its own derived from the
// project identity, because the host records a single path per name.
func MarketplaceName(scope, projectID string) string {
	id := []rune(projectID)
	if scope != scopeProject || len(id) < 12 {
		return userMarketplace
	}
	return userMarketplace + "-" + string(id[:12])
}

// pointerKeys lists the settings fields an installation wrote, the two of
// a receipt written before the project marketplace had its own name.
func pointerKeys(owned Ownership) []string {
	if len(owned.Installed) == 0 {
		return []string{"enabledPlugins/golaw@golaw", "extraKnownMarketplaces/golaw"}
	}
	return slices.Sorted(maps.Keys(owned.Installed))
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
