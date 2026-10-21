package policyfiles

type goModule struct {
	path     string
	direct   []moduleRequirement
	replaces []moduleReplacement
}

type moduleRequirement struct {
	path, version string
	line          int
}

type moduleReplacement struct {
	oldPath, oldVersion string
	newPath, newVersion string
	line                int
}

// readModule uses parsed module, require and replace directives so every block
// follows Go syntax and only indirect markers determine which requires to omit.
// Without a module directive, the dependency graph cannot be identified.
func readModule(path string, data []byte) (goModule, error) {
	f, err := parseModule(path, data)
	if err != nil {
		return goModule{}, err
	}
	if f.Module == nil {
		return goModule{}, outside(path, 1, "missing module directive")
	}
	m := goModule{path: f.Module.Mod.Path}
	for _, r := range f.Require {
		if r.Indirect {
			continue
		}
		m.direct = append(m.direct, moduleRequirement{
			path: r.Mod.Path, version: r.Mod.Version, line: r.Syntax.Start.Line,
		})
	}
	for _, r := range f.Replace {
		m.replaces = append(m.replaces, moduleReplacement{
			oldPath: r.Old.Path, oldVersion: r.Old.Version,
			newPath: r.New.Path, newVersion: r.New.Version,
			line: r.Syntax.Start.Line,
		})
	}
	return m, nil
}
