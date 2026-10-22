package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/BurntSushi/toml"
)

// codexTrustConfig is the part of the user config.toml of Codex that
// records hook trust; every other table and key of the file is ignored.
type codexTrustConfig struct {
	Hooks struct {
		State map[string]struct {
			TrustedHash *string `toml:"trusted_hash"`
		} `toml:"state"`
	} `toml:"hooks"`
}

// CodexTrusted reads the hook trust Codex recorded in its user config.toml:
// the trusted_hash of every [hooks.state."<key>"] table, by key, whether
// the handler is enabled or not, which doctor judges apart. It never
// writes: the /hooks review of Codex is the one writer of that table, so
// the person approves exactly the definition on disk. An absent file is an
// empty record; a file Codex itself would refuse is an error.
func CodexTrusted(configPath string) (map[string]string, error) {
	data, err := os.ReadFile(configPath) //nolint:gosec // The installer names the configuration of the host.
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the hook trust of %s: %w", configPath, err)
	}
	var config codexTrustConfig
	if _, err := toml.Decode(string(data), &config); err != nil {
		return nil, fmt.Errorf("read the hook trust of %s: %w: %w", configPath, ErrSettings, err)
	}
	trusted := make(map[string]string, len(config.Hooks.State))
	for key, state := range config.Hooks.State {
		if state.TrustedHash != nil {
			trusted[key] = *state.TrustedHash
		}
	}
	return trusted, nil
}
