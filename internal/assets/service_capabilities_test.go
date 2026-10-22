package assets_test

import (
	"path"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/model"
)

// directRequires reads the direct requires of the rendered go.mod, which
// init records in the inherited baseline before it renders the core
// templates over an adopter: the generated API code brings its own.
func directRequires(rendered []assets.Rendered) []assets.Requirement {
	var requires []assets.Requirement
	block := false
	for i := range rendered {
		if rendered[i].Path != "go.mod" {
			continue
		}
		for line := range strings.SplitSeq(string(rendered[i].Data), "\n") {
			fields := strings.Fields(line)
			switch {
			case len(fields) == 2 && fields[0] == "require" && fields[1] == "(":
				block = true
			case len(fields) == 1 && fields[0] == ")":
				block = false
			case block && len(fields) == 2:
				requires = append(requires, assets.Requirement{Path: fields[0], Version: fields[1]})
			case !block && len(fields) == 3 && fields[0] == "require":
				requires = append(requires, assets.Requirement{Path: fields[1], Version: fields[2]})
			}
		}
	}
	return requires
}

// namedPaths answers every path the rendered db and api configurations
// name: the sqlc schema, queries and Go output, the Redocly contract root
// and the oapi-codegen outputs.
func namedPaths(t *testing.T, files map[string][]byte) []string {
	t.Helper()
	var sqlc struct {
		SQL []struct {
			Schema  any `yaml:"schema"`
			Queries any `yaml:"queries"`
			Gen     struct {
				Go struct {
					Out string `yaml:"out"`
				} `yaml:"go"`
			} `yaml:"gen"`
		} `yaml:"sql"`
	}
	if err := yaml.Unmarshal(files["sqlc.yaml"], &sqlc); err != nil {
		t.Fatal(err)
	}
	var named []string
	for _, entry := range sqlc.SQL {
		named = append(named, scalarPaths(entry.Schema)...)
		named = append(named, scalarPaths(entry.Queries)...)
		if entry.Gen.Go.Out != "" {
			named = append(named, entry.Gen.Go.Out)
		}
	}
	var redocly struct {
		APIs map[string]struct {
			Root string `yaml:"root"`
		} `yaml:"apis"`
	}
	if err := yaml.Unmarshal(files["redocly.yaml"], &redocly); err != nil {
		t.Fatal(err)
	}
	for _, api := range redocly.APIs {
		named = append(named, api.Root)
	}
	return append(named, codegenOutputs(t, files)...)
}

// codegenOutputs answers the output of every rendered oapi-codegen
// configuration.
func codegenOutputs(t *testing.T, files map[string][]byte) []string {
	t.Helper()
	var named []string
	for name, data := range files {
		if !strings.HasPrefix(name, "api/codegen/") || !strings.HasSuffix(name, ".yaml") {
			continue
		}
		var codegen struct {
			Output string `yaml:"output"`
		}
		if err := yaml.Unmarshal(data, &codegen); err != nil {
			t.Fatal(err)
		}
		if codegen.Output != "" {
			named = append(named, codegen.Output)
		}
	}
	return named
}

// scalarPaths reads a sqlc path field, a string or a list of strings.
func scalarPaths(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case []any:
		var out []string
		for _, item := range typed {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	}
	return nil
}

func TestServiceFixtureHoldsWhatTheDbAndApiTemplatesName(t *testing.T) {
	t.Parallel()
	files := serviceFiles(t, model.Capabilities())
	named := namedPaths(t, files)
	if len(named) == 0 {
		t.Fatal("the rendered db and api configurations name no path")
	}
	for _, want := range append(named, "api/openapi.base.yaml") {
		clean := path.Clean(strings.TrimPrefix(want, "./"))
		found := false
		for name := range files {
			if name == clean || strings.HasPrefix(name, clean+"/") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the service fixture lacks %s, which a rendered configuration names", clean)
		}
	}
}

// requireInModule adds a direct require where go mod tidy keeps it: in the
// first require block, in path order, when the go.mod has one, and as a
// statement of its own otherwise.
func requireInModule(gomod []byte, module string) []byte {
	lines := strings.Split(string(gomod), "\n")
	start := slices.Index(lines, "require (")
	if start < 0 {
		return append(slices.Clone(gomod), "\nrequire "+module+"\n"...)
	}
	at := start + 1
	for lines[at] != ")" && lines[at] < "\t"+module {
		at++
	}
	return []byte(strings.Join(slices.Insert(lines, at, "\t"+module), "\n"))
}

// mergedSum adds the go.sum lines of one module where go mod tidy keeps
// them: lines sort by module path, then by version, so the lines of a
// module the file lacks go before the first line of a greater path.
func mergedSum(existing []byte, extra string) []byte {
	var lines []string
	for line := range strings.SplitSeq(string(existing), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	for line := range strings.SplitSeq(extra, "\n") {
		if line == "" {
			continue
		}
		module, _, _ := strings.Cut(line, " ")
		at := slices.IndexFunc(lines, func(l string) bool { name, _, _ := strings.Cut(l, " "); return name > module })
		if at < 0 {
			at = len(lines)
		}
		lines = slices.Insert(lines, at, line)
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}
