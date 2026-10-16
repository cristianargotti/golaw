package coexist

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Where the Python gate registers itself on the machine: the Claude Code
// user settings carry its hooks, the registry beside them lists the
// repositories it governs and the flags of its families.
const (
	settingsFile = "settings.json"
	registryFile = "pcc-standards.json"
	gateScript   = "pcc_gate.py"
)

// Detection is what the machine and the repository say about the gate.
type Detection struct {
	Registered map[string]bool // the events a gate hook answers
	Governs    bool            // the repository is in the gate's registry
	Released   map[string]bool // the families the registry switched off
}

// registry is the shape of the gate registry the detection reads: the
// governed repositories and one switch per shared family, absent or true
// while the gate keeps the family.
type registry struct {
	Repos      []string `json:"repos"`
	Text       *bool    `json:"text_checks"`
	Paths      *bool    `json:"paths_checks"`
	Sizes      *bool    `json:"sizes_checks"`
	SecretRead *bool    `json:"secret_read_checks"`
	Shell      *bool    `json:"shell_checks"`
}

// Detect reads the registration of the gate in the configuration directory
// of the agent, where the host reads its user settings (the directory an
// override names, or the default one under home), and answers whether it
// governs the repository at root; home expands a ~ of the registry. A
// missing file means no gate; a file that cannot be decoded is an error.
func Detect(home, configDir, root string) (Detection, error) {
	d := Detection{Registered: map[string]bool{}, Released: map[string]bool{}}
	var s settings
	if err := decode(SettingsPath(configDir), &s); err != nil {
		return Detection{}, err
	}
	for event, groups := range s.Hooks {
		for _, group := range groups {
			if !group.registersGate() {
				continue
			}
			if err := markEvents(d.Registered, event, group.Matcher); err != nil {
				return Detection{}, err
			}
		}
	}
	var r registry
	if err := decode(RegistryPath(configDir), &r); err != nil {
		return Detection{}, err
	}
	d.Governs = governs(home, r.Repos, root)
	d.Released = r.released()
	return d, nil
}

// SettingsPath is the user settings file in the configuration directory of
// the agent, where the gate registers its hooks.
func SettingsPath(configDir string) string { return filepath.Join(configDir, settingsFile) }

// RegistryPath is the registry of the gate beside the settings: the
// repositories it governs and the switches of its families.
func RegistryPath(configDir string) string { return filepath.Join(configDir, registryFile) }

// decode reads a JSON file into v; a missing file leaves v untouched.
func decode(path string, v any) error {
	data, err := os.ReadFile(path) //nolint:gosec // the path is a registration file under the person's home
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrDetect, filepath.Base(path), err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%w: %s: %w", ErrDetect, filepath.Base(path), err)
	}
	return nil
}

// governs reports whether root is one of the governed repositories or a
// directory inside one. Both sides take the form the gate compares, so a
// relative, linked or home-relative path matches as it does for the gate.
func governs(home string, repos []string, root string) bool {
	target, ok := realPath(home, root)
	if !ok {
		return false
	}
	for _, repo := range repos {
		governed, ok := realPath(home, repo)
		if !ok {
			continue
		}
		if target == governed || strings.HasPrefix(target, governed+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// realPath is the absolute path with every link resolved, a leading ~
// standing for the home: the realpath of expanduser the gate applies to
// its registry and to the path it judges.
func realPath(home, path string) (string, bool) {
	if rest, ok := strings.CutPrefix(path, "~"); ok && (rest == "" || os.IsPathSeparator(rest[0])) {
		path = filepath.Join(home, rest)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(abs)
	return resolved, err == nil
}

// released lists the families whose switch is explicitly off.
func (r registry) released() map[string]bool {
	flags := map[string]*bool{
		FamilyText: r.Text, FamilyPaths: r.Paths, FamilySizes: r.Sizes,
		FamilySecretRead: r.SecretRead, FamilyShell: r.Shell,
	}
	out := map[string]bool{}
	for family, flag := range flags {
		if flag != nil && !*flag {
			out[family] = true
		}
	}
	return out
}

// owns reports whether the gate keeps the family on this repository.
func (d Detection) owns(family string) bool {
	if !d.Governs || d.Released[family] {
		return false
	}
	return slices.ContainsFunc(judged()[family], func(event string) bool { return d.Registered[event] })
}

// summary is the first line of the table.
func (d Detection) summary() string {
	events := make([]string, 0, len(d.Registered))
	for _, event := range []string{EventWrite, EventRead, EventShell, EventPostWrite, EventPostShell} {
		if d.Registered[event] {
			events = append(events, event)
		}
	}
	if len(events) == 0 {
		return "gate: no hook registered on this machine"
	}
	scope := "does not govern this repository"
	if d.Governs {
		scope = "governs this repository"
	}
	return "gate: registered for " + strings.Join(events, ", ") + "; " + scope
}
