package gentest

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"strings"

	"github.com/cristianargotti/golaw/internal/proof"
)

// growthRoot is the embedded directory of the fixture; a key of Contracts is
// the slash path under it, the path the file has in an adopter's repository.
const growthRoot = "testdata/growth"

// growth holds the fixture inside the test binary, so every package that
// reads it gets the same bytes wherever its test runs from.
//
//go:embed testdata/growth
var growth embed.FS

// Contracts are the fixture contracts every kind, golden slice and
// acceptance row reads, by slash path: the growth area of M7.3.2 with its
// ports, its migration and queries, its OpenAPI document and the recorded
// outputs of the pinned sqlc and oapi-codegen over them. A .esc file is
// decoded and loses its suffix, so go.mod is stored without making the
// fixture a nested module. Every call answers new bytes, so a test that
// edits its copy never changes what another test reads.
func Contracts() map[string][]byte {
	files := make(map[string][]byte)
	err := fs.WalkDir(growth, growthRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		content, err := growth.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		rel := strings.TrimPrefix(path, growthRoot+"/")
		if name, ok := strings.CutSuffix(rel, proof.EscapedSuffix); ok {
			rel, content = name, proof.Unescape(content)
		}
		files[rel] = bytes.Clone(content)
		return nil
	})
	if err != nil {
		// The tree is compiled into the binary, so a failed walk is a broken
		// build of this package and no test could run on a partial fixture.
		panic(fmt.Errorf("gentest: read the embedded growth fixture: %w", err))
	}
	return files
}
