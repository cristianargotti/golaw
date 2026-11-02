package goldentest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/proof"
)

// proofManifest is a kind's testdata/proof.yml: each entry binds one law
// to a red tree it must reject and a near tree it must accept.
type proofManifest struct {
	Entries []proofEntry `yaml:"entries"`
}

type proofEntry struct {
	Law  string `yaml:"law"`
	Red  string `yaml:"red"`
	Near string `yaml:"near"`
}

// Prove fails the test with every reason the manifest proves nothing.
func Prove(t *testing.T, manifestPath string, laws []Law) {
	t.Helper()
	for _, failure := range Verify(manifestPath, laws) {
		t.Errorf("%s", failure)
	}
}

// Verify answers why the manifest does not prove its laws both ways: a
// law that rejects nothing would let any rendering through, and one that
// rejects its near tree would refuse a sound template.
func Verify(manifestPath string, laws []Law) []string {
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return []string{manifestPath + ": " + err.Error()}
	}
	if len(manifest.Entries) == 0 {
		return []string{manifestPath + ": no entry"}
	}
	base := filepath.Dir(manifestPath)
	var out []string
	for _, entry := range manifest.Entries {
		out = append(out, verifyEntry(manifestPath, base, entry, laws)...)
	}
	return out
}

func readManifest(path string) (proofManifest, error) {
	file, err := os.Open(path) //nolint:gosec // the manifest path is the test's own
	if err != nil {
		return proofManifest{}, fmt.Errorf("%w: %w", ErrProof, err)
	}
	defer func() { _ = file.Close() }()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var manifest proofManifest
	if err := decoder.Decode(&manifest); err != nil {
		return proofManifest{}, fmt.Errorf("%w: %w", ErrProof, err)
	}
	return manifest, nil
}

func verifyEntry(manifestPath, base string, entry proofEntry, laws []Law) []string {
	law, ok := lawByName(laws, entry.Law)
	if !ok {
		return []string{fmt.Sprintf("%s: law %s is not a template law", manifestPath, entry.Law)}
	}
	var out []string
	red, err := tree(filepath.Join(base, filepath.FromSlash(entry.Red)))
	switch {
	case err != nil:
		out = append(out, fmt.Sprintf("%s: red %s: %v", manifestPath, entry.Red, err))
	case len(law.Assert(red)) == 0:
		out = append(out, fmt.Sprintf("%s: red %s does not violate %s", manifestPath, entry.Red, law.Name))
	}
	near, err := tree(filepath.Join(base, filepath.FromSlash(entry.Near)))
	if err != nil {
		return append(out, fmt.Sprintf("%s: near %s: %v", manifestPath, entry.Near, err))
	}
	if messages := law.Assert(near); len(messages) != 0 {
		out = append(out, fmt.Sprintf("%s: near %s violates %s: %s", manifestPath, entry.Near, law.Name, messages[0]))
	}
	return out
}

func lawByName(laws []Law, name string) (Law, bool) {
	for _, law := range laws {
		if law.Name == name {
			return law, true
		}
	}
	return Law{}, false
}

// tree reads a rendered tree keyed by slash path relative to it; an
// escaped file is decoded and loses its suffix, so a fixture the kit's own
// checks would flag is judged as the rendering it stands for.
func tree(dir string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("relative path of %s: %w", path, err)
		}
		content, err := os.ReadFile(path) //nolint:gosec // the fixture path is the test's own
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		key := filepath.ToSlash(rel)
		if trimmed, escaped := strings.CutSuffix(key, proof.EscapedSuffix); escaped {
			key, content = trimmed, proof.Unescape(content)
		}
		files[key] = content
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk tree: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: the tree holds no file", ErrProof)
	}
	return files, nil
}
