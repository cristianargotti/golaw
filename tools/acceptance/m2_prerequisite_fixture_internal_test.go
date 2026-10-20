package main

import (
	"path/filepath"
	"testing"
)

// Simulated rows need neither host tool directories nor symlink privileges;
// dedicated prerequisite tests retain the real directory builder.
func m2SimulatedPrerequisites(s *suite, _, bin string) ([]string, error) {
	dir := filepath.Join(s.base, "prerequisites")
	for _, name := range []string{"go", m2GitCommand, "sh", binaryName} {
		if err := writeFile(filepath.Join(dir, executable(name)), []byte("simulated "+name+"\n"), execMode); err != nil {
			return nil, err
		}
	}
	return m2DistinctDirectories([]string{bin, filepath.Dir(s.binary), dir}), nil
}

func TestM2SimulatedPrerequisitesStayInTheirScope(t *testing.T) {
	t.Parallel()
	for _, system := range []string{"linux", "darwin", goosWindows} {
		s := m2TestSuite(t).m2Scope(system)
		s.m2OS = system
		env, err := s.m2PrerequisiteEnvironment(s.env, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"go", m2GitCommand, "sh", binaryName} {
			path, err := resolveExecutable(name, env)
			if err != nil || (filepath.Dir(path) != filepath.Join(s.base, "prerequisites") && path != s.binary) {
				t.Fatalf("%s %s resolved outside the fixture: %s %v", system, name, path, err)
			}
		}
		if err := m2MissingTools(env); err != nil {
			t.Fatal(err)
		}
	}
}
