package doctor

import (
	"encoding/json"
	"slices"
)

type settingsDocument struct {
	EnabledPlugins map[string]bool `json:"enabledPlugins"`
	Permissions    struct {
		Deny []string `json:"deny"`
	} `json:"permissions"`
}

func pluginLines(snapshot Snapshot) []Line {
	var settings settingsDocument
	err := json.Unmarshal([]byte(snapshot.Settings), &settings)
	line := Line{ID: "doctor.plugin", State: stateOK, Code: "plugin-enabled"}
	if err != nil || !settings.EnabledPlugins["golaw@golaw"] {
		line.State, line.Code = stateRed, "plugin-disabled"
	}
	line.Detail = "configured enablement; live dispatch is not established"
	return []Line{line}
}

func readLines(snapshot Snapshot) []Line {
	var settings settingsDocument
	err := json.Unmarshal([]byte(snapshot.Settings), &settings)
	line := Line{ID: "doctor.read", State: stateOK, Code: "read-deny-present"}
	if err != nil || len(snapshot.RequiredRead) == 0 {
		line.State, line.Code = stateRed, "read-deny-missing"
	}
	for _, rule := range snapshot.RequiredRead {
		if !slices.Contains(settings.Permissions.Deny, rule) {
			line.State, line.Code = stateRed, "read-deny-missing"
			line.Detail = rule
			break
		}
	}
	return []Line{line}
}

func trustLines(snapshot Snapshot) []Line {
	line := Line{
		ID: "doctor.trust", State: stateUnknown,
		Code:   "plugin-hook-trust-unproven",
		Detail: "plugin hook trust and live dispatch are unknown",
	}
	var plugin struct {
		Name string `json:"name"`
	}
	if json.Unmarshal([]byte(snapshot.Plugin), &plugin) != nil ||
		plugin.Name != "golaw" || pluginLines(snapshot)[0].State != stateOK {
		line.Code = "plugin-unavailable"
	}
	return []Line{line}
}
