package profile

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

// SonarWayDir is the optional directory of M6.3.3 that holds a dated
// snapshot of a company server's Sonar way activation, one file per
// language.
const SonarWayDir = "sonar-way"

// sonarRuleKey is a Sonar rule key without its language.
var sonarRuleKey = regexp.MustCompile(`^S\d+$`)

// sonarLanguages are the Sonar language keys a snapshot file may name.
func sonarLanguages() []string {
	return []string{"cloudformation", "css", "docker", "js", "py", "ts", "web"}
}

// sonarWayFile is sonar-way/<language>.yml as the file holds it.
type sonarWayFile struct {
	Read  string          `yaml:"read"`
	Rules []sonarWayEntry `yaml:"rules"`
}

type sonarWayEntry struct {
	Key      string            `yaml:"key"`
	Severity string            `yaml:"severity"`
	Params   map[string]string `yaml:"params"`
}

// errNotDirectory names a snapshot path that holds a file.
var errNotDirectory = errors.New("not a directory")

// readSonarWay reads the snapshot of a bundle directory: every file one
// language, every rule a key, a severity and its parameters, sorted by
// language and key; the snapshot is as fresh as its oldest file, so Read
// is the earliest date. A bundle without the directory, or with an empty
// one, answers false.
func readSonarWay(dir, name string) (model.SonarWay, bool, error) {
	root := filepath.Join(dir, SonarWayDir)
	// Windows lists a file where the directory belongs as absent or empty,
	// so the kind is read first and a file never passes for no snapshot.
	info, err := os.Stat(root)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return model.SonarWay{}, false, nil
	case err == nil && !info.IsDir():
		err = errNotDirectory
	}
	var entries []os.DirEntry
	if err == nil {
		entries, err = os.ReadDir(root)
	}
	if err != nil {
		return model.SonarWay{}, false, bundlef("profile %s: "+readCause, name, SonarWayDir, err)
	}
	var out model.SonarWay
	for _, entry := range entries {
		rules, read, err := readSonarWayFile(root, name, entry)
		if err != nil {
			return model.SonarWay{}, false, err
		}
		out.Rules = append(out.Rules, rules...)
		if out.Read == "" || read < out.Read {
			out.Read = read
		}
	}
	slices.SortFunc(out.Rules, func(a, b model.SonarWayRule) int {
		return cmp.Or(cmp.Compare(a.Language, b.Language), cmp.Compare(a.Key, b.Key))
	})
	return out, len(entries) > 0, nil
}

// readSonarWayFile reads and judges one language file of the snapshot.
func readSonarWayFile(root, name string, entry fs.DirEntry) ([]model.SonarWayRule, string, error) {
	file := SonarWayDir + "/" + entry.Name()
	language, isYAML := strings.CutSuffix(entry.Name(), ".yml")
	if entry.IsDir() || !isYAML || !slices.Contains(sonarLanguages(), language) {
		return nil, "", bundlef("profile %s: %s: name a snapshot file <language>.yml with one of %s",
			name, file, strings.Join(sonarLanguages(), ", "))
	}
	raw, err := os.ReadFile(filepath.Join(root, entry.Name())) //nolint:gosec // the path is an entry of the verified bundle directory
	if err != nil {
		return nil, "", bundlef("profile %s: "+readCause, name, file, err)
	}
	var f sonarWayFile
	if cause := decodeStrict(raw, &f); cause != "" {
		return nil, "", bundlef(fileCause, name, file, cause)
	}
	if cause := f.invalid(); cause != "" {
		return nil, "", bundlef(fileCause, name, file, cause)
	}
	rules := make([]model.SonarWayRule, 0, len(f.Rules))
	for _, r := range f.Rules {
		rules = append(rules, model.SonarWayRule{Language: language, Key: r.Key, Severity: r.Severity, Params: r.Params})
	}
	return rules, f.Read, nil
}

// invalid answers the first value the file may not hold: a read date that
// is not YYYY-MM-DD, a key that is not S and digits, a key named twice, an
// empty severity.
func (f sonarWayFile) invalid() string {
	if _, err := time.Parse(time.DateOnly, f.Read); err != nil {
		return fmt.Sprintf("read: %q is not a date like 2026-10-27", f.Read)
	}
	for i, r := range f.Rules {
		switch {
		case !sonarRuleKey.MatchString(r.Key):
			return fmt.Sprintf("rules: key %q is not a Sonar rule key like S2681", r.Key)
		case slices.ContainsFunc(f.Rules[:i], func(other sonarWayEntry) bool { return other.Key == r.Key }):
			return fmt.Sprintf("rules: key %s is named twice", r.Key)
		case strings.TrimSpace(r.Severity) == "":
			return fmt.Sprintf("rules: key %s has no severity", r.Key)
		}
	}
	return ""
}
