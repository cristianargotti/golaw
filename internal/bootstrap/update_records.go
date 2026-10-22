package bootstrap

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/install"
)

// The baseline decision record init renders, and the tools module of the
// kit whose requires a later version measures.
const (
	baselineRecord = "0001-inherited-baseline.md"
	toolsModule    = "tools/go.mod"
)

// addRecordModules proposes into the modules of the baseline record the
// modules that the kit's go.mod lines and tools/go.mod require and that no
// decision record holds, because a version that starts measuring the tools
// module must not leave the pipeline red for the files the kit wrote, while
// the record stays the person's to apply. A record in a shape no earlier
// init wrote is left alone, and the check names what it misses.
func (plan *TemplatePlan) addRecordModules(files map[string]assets.Rendered) error {
	record := ""
	for name := range files {
		if path.Base(name) == baselineRecord && files[name].Template.Ownership == assets.OwnedByDeveloper {
			record = name
		}
	}
	if record == "" {
		return nil
	}
	before, existed, err := readTemplate(plan.root, record)
	if err != nil || !existed {
		return err
	}
	recorded, err := recordedModules(plan.root, path.Dir(record))
	if err != nil {
		return err
	}
	var missing []string
	for _, module := range kitModules(files) {
		if !recorded[module] && !slices.Contains(missing, module) {
			missing = append(missing, module)
		}
	}
	if after, ok := withModules(before, missing); len(missing) != 0 && ok {
		plan.Proposed = append(plan.Proposed, install.Change{Path: record, Before: before, After: after, Existed: true})
	}
	return nil
}

// kitModules lists the requires of the kit's go.mod lines, then of its
// tools module, in their order.
func kitModules(files map[string]assets.Rendered) []string {
	var modules []string
	for _, name := range []string{goModule, toolsModule} {
		file, found := files[name]
		if !found {
			continue
		}
		parsed, err := modfile.ParseLax(name, file.Data, nil)
		if err != nil {
			continue
		}
		for _, require := range parsed.Require {
			modules = append(modules, require.Mod.Path)
		}
	}
	return modules
}

// recordedModules reads the modules every decision record of the directory
// lists; a record whose front matter does not parse records none.
func recordedModules(root, directory string) (map[string]bool, error) {
	recorded := make(map[string]bool)
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(directory)), func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(name), ".md") {
			return err
		}
		data, err := os.ReadFile(name) //nolint:gosec // the walk stays below the decisions directory of the root
		if err != nil {
			return fmt.Errorf("bootstrap: read decision record: %w", err)
		}
		modules, _ := frontModules(data)
		for _, module := range modules {
			recorded[module] = true
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("bootstrap: read the decision records: %w", err)
	}
	return recorded, nil
}

// frontModules decodes the modules of the front matter that opens a record.
func frontModules(data []byte) ([]string, bool) {
	lines := strings.Split(string(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))), "\n")
	if strings.TrimSuffix(lines[0], "\r") != frontDelimiter {
		return nil, false
	}
	for index := 1; index < len(lines); index++ {
		if strings.TrimSuffix(lines[index], "\r") == frontDelimiter {
			var front struct {
				Modules []string `yaml:"modules"`
			}
			err := yaml.Unmarshal([]byte(strings.Join(lines[1:index], "\n")), &front)
			return front.Modules, err == nil
		}
	}
	return nil, false
}
