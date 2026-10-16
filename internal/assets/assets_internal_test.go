package assets

import (
	"bytes"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestEmbeddedManifestAndSkill(t *testing.T) {
	t.Parallel()
	files, err := Files()
	if err != nil {
		t.Fatal(err)
	}
	skills := make([][]byte, 0, 2)
	for _, file := range files {
		data, readErr := Read(file.Path)
		if readErr != nil || !bytes.Equal(data, file.Data) {
			t.Fatalf("asset differs: %s: %v", file.Path, readErr)
		}
		if strings.HasSuffix(file.Path, "/SKILL.md") {
			skills = append(skills, file.Data)
		}
	}
	if len(skills) != 2 || !bytes.Equal(skills[0], skills[1]) {
		t.Fatal("skill source and copy differ")
	}
	if len(skills[0]) > 10000 {
		t.Fatal("skill exceeds byte budget")
	}
	if bytes.Count(bytes.TrimSuffix(skills[0], []byte("\n")), []byte("\n"))+1 > 100 {
		t.Fatal("skill exceeds line budget")
	}
	if _, err := Read("agents/absent"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing asset: %v", err)
	}
}

func TestManifestRejectsIncompleteAndInvalidTrees(t *testing.T) {
	t.Parallel()
	cases := []string{
		`{`,
		`{"files":[],"unknown":true}`,
		`{"files":[]} {}`,
		`{"files":[]}`,
		`{"files":["../outside"]}`,
		`{"files":["other/file"]}`,
		`{"files":["agents/a","agents/a"]}`,
		`{"files":["agents/missing"]}`,
		`{"files":["templates/manifest.yml","agents/a"]}`,
		`{"files":["agents/a","agents/manifest.json"]}`,
	}
	for _, document := range cases {
		t.Run(document, func(t *testing.T) {
			t.Parallel()
			tree := manifestTree(document)
			if _, err := Load(tree); !errors.Is(err, ErrManifest) {
				t.Fatalf("expected invalid manifest, got %v", err)
			}
		})
	}
}

func TestManifestAcceptsCompleteTree(t *testing.T) {
	t.Parallel()
	document := `{"files":["agents/a","agents/manifest.json","templates/manifest.yml"]}`
	files, err := Load(manifestTree(document))
	if err != nil || len(files) != 3 {
		t.Fatalf("complete manifest: %d: %v", len(files), err)
	}
	if _, err := Load(fstest.MapFS{}); !errors.Is(err, ErrManifest) {
		t.Fatalf("absent manifest: %v", err)
	}
	tree := manifestTree(`{"files":["agents/a","agents/manifest.json"]}`)
	delete(tree, "templates/manifest.yml")
	if _, err := Load(tree); !errors.Is(err, ErrManifest) {
		t.Fatalf("absent root: %v", err)
	}
}

func manifestTree(document string) fstest.MapFS {
	return fstest.MapFS{
		"agents/a":               {Data: []byte("asset\n")},
		"agents/manifest.json":   {Data: []byte(document)},
		"templates/manifest.yml": {Data: []byte("templates: []\n")},
	}
}
