package install

import (
	"fmt"
	"path/filepath"
)

// codexTrustNote is the step the person takes after an install whose
// handlers Codex does not trust yet, because Codex skips an untrusted or
// changed hook without a word.
const codexTrustNote = "Abra /hooks no Codex e valide os hooks do golaw."

// codexTrustNotes adds the /hooks step when the hash of any handler the
// receipt lists differs from the one Codex recorded for its key, or none
// is recorded. Codex reads hook trust from the user config.toml alone, so
// the record is read there for a project install too.
func codexTrustNotes(plan *Plan, options Options) error {
	if !filepath.IsAbs(options.ConfigDir) {
		return fmt.Errorf("codex configuration directory: %w", ErrPath)
	}
	trusted, err := CodexTrusted(filepath.Join(options.ConfigDir, "config.toml"))
	if err != nil {
		return err
	}
	for _, entry := range plan.Receipt.Entries {
		hash, err := CodexTrustHash(entry.Event, entry.Matcher, entry.Definition, options.GOOS)
		if err != nil {
			return err
		}
		key := CodexTrustKey(plan.Receipt.HooksPath, entry.Event, entry.Group, entry.Handler)
		if trusted[key] != hash {
			plan.Notes = append(plan.Notes, codexTrustNote)
			return nil
		}
	}
	return nil
}
