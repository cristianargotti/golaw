package bootstrap_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
)

func TestSuggestNamesTheCapabilitiesTheFilesPointAt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		files   []string
		enabled []string
		want    []bootstrap.Suggestion
	}{
		{
			"red template birth",
			[]string{"Dockerfile", "api/swagger.yaml", "api/docs.go"},
			[]string{"go"},
			[]bootstrap.Suggestion{{Capability: "api", Evidence: "api/swagger.yaml"}, {Capability: "image", Evidence: "Dockerfile"}},
		},
		{
			"red migrations and a named Dockerfile",
			[]string{"db/migrations/0001_init.sql", "deploy/service.Dockerfile"},
			[]string{"go"},
			[]bootstrap.Suggestion{{Capability: "db", Evidence: "db/migrations/0001_init.sql"}, {Capability: "image", Evidence: "deploy/service.Dockerfile"}},
		},
		{
			"red sqlc and a hexagonal layout",
			[]string{"sqlc.yaml", "internal/domain/item.go", "internal/application/catalog.go"},
			[]string{"go"},
			[]bootstrap.Suggestion{{Capability: "db", Evidence: "sqlc.yaml"}, {Capability: "hexagonal", Evidence: "internal/domain"}},
		},
		{"near capabilities already enabled", []string{"Dockerfile", "openapi.json"}, []string{"api", "go", "image"}, nil},
		{"near fixtures, vendored and hidden files", []string{"testdata/Dockerfile", "vendor/x/openapi.yaml", ".docker/Dockerfile", "node_modules/a/swagger.json"}, []string{"go"}, nil},
		{"near names that only resemble", []string{"docs/swagger.md", "migrations/README.md", "internal/domain/item.go", "Dockerfiles/readme.txt"}, []string{"go"}, nil},
		{"near openapi without go", []string{"openapi.yaml"}, nil, nil},
		{"near sqlc without go", []string{"sqlc.yaml"}, nil, nil},
		{"near migrations without go", []string{"db/migrations/0001_init.sql"}, nil, nil},
		{"near hexagonal without go", []string{"internal/domain/item.go", "internal/application/catalog.go"}, nil, nil},
		{"red image without go", []string{"Dockerfile"}, nil, []bootstrap.Suggestion{{Capability: "image", Evidence: "Dockerfile"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			for _, path := range tc.files {
				writeBefore(t, filepath.Join(root, filepath.FromSlash(path)), "x\n")
			}
			got, err := bootstrap.Suggest(root, tc.enabled)
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("Suggest = %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}

func TestSuggestMatchesOnlyTheDeclaredFileNames(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ path, capability string }{
		{"api/openapi.yml", "api"},
		{"api/openapi.yaml", "api"},
		{"api/swagger.json", "api"},
		{"api/swagger.yml", "api"},
		{"db/sqlc.yml", "db"},
		{"db/sqlc.json", "db"},
		{"db/migrations/nested/001.sql", "db"},
		{"deploy/Dockerfile.prod", "image"},
		{"deploy/worker.Dockerfile", "image"},
		{"Dockerfile.", ""},
		{".Dockerfile", ""},
		{"dockerfile", ""},
		{"api/OpenAPI.yaml", ""},
		{"api/openapi.yaml.bak", ""},
		{"api/swagger.txt", ""},
		{"db/sqlc.toml", ""},
		{"db/my-migrations/001.sql", ""},
		{"db/migrations.sql", ""},
		{"db/migrations/001.SQL", ""},
	} {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			if runtime.GOOS == "windows" && strings.HasSuffix(tc.path, ".") {
				t.Skip("Windows drops the trailing dot of a file name, so the file written would be another name")
			}
			root := t.TempDir()
			writeBefore(t, filepath.Join(root, filepath.FromSlash(tc.path)), "x\n")
			var want []bootstrap.Suggestion
			if tc.capability != "" {
				want = []bootstrap.Suggestion{{Capability: tc.capability, Evidence: tc.path}}
			}
			got, err := bootstrap.Suggest(root, []string{"go"})
			if err != nil || !slices.Equal(got, want) {
				t.Fatalf("Suggest = %+v, %v; want %+v", got, err, want)
			}
		})
	}
}

func TestSuggestKeepsFirstEvidenceAndTheEnabledSlice(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, path := range []string{
		"z/Dockerfile", "z/openapi.json", "z/sqlc.yaml",
		"a/Dockerfile", "a/swagger.yaml", "a/migrations/001.sql",
		"internal/domain/item.go", "internal/application/service.go",
	} {
		writeBefore(t, filepath.Join(root, filepath.FromSlash(path)), "x\n")
	}
	storage := []string{"go", "cli", "sentinel", "sentinel", "sentinel", "sentinel"}
	before := slices.Clone(storage)
	want := []bootstrap.Suggestion{
		{Capability: "api", Evidence: "a/swagger.yaml"},
		{Capability: "db", Evidence: "a/migrations/001.sql"},
		{Capability: "hexagonal", Evidence: "internal/domain"},
		{Capability: "image", Evidence: "a/Dockerfile"},
	}
	got, err := bootstrap.Suggest(root, storage[:2])
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("Suggest = %+v, %v; want %+v", got, err, want)
	}
	if !slices.Equal(storage, before) {
		t.Fatalf("enabled backing array changed: %v, want %v", storage, before)
	}
}

func TestSuggestNeverSkipsTheRootName(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"testdata", "vendor", "node_modules", ".hidden"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(t.TempDir(), name)
			for _, path := range []string{
				"Dockerfile", "nested/testdata/openapi.yaml", "nested/vendor/openapi.yaml",
				"nested/node_modules/sqlc.yaml", "nested/.hidden/sqlc.yaml",
			} {
				writeBefore(t, filepath.Join(root, filepath.FromSlash(path)), "x\n")
			}
			got, err := bootstrap.Suggest(root+string(filepath.Separator), []string{"go"})
			want := []bootstrap.Suggestion{{Capability: "image", Evidence: "Dockerfile"}}
			if err != nil || !slices.Equal(got, want) {
				t.Fatalf("Suggest = %+v, %v; want %+v", got, err, want)
			}
		})
	}
}

func TestSuggestIgnoresNamedDirectoriesAndSymbolicLinks(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need privileges on Windows")
	}
	root, outside := t.TempDir(), t.TempDir()
	for _, name := range []string{"Dockerfile", "openapi.yaml", "sqlc.json", "migrations/001.sql"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(name)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"Dockerfile", "swagger.yml", "sqlc.yaml"} {
		writeBefore(t, filepath.Join(outside, name), "x\n")
	}
	for _, name := range []string{"service.Dockerfile", "swagger.json", "sqlc.yml"} {
		if err := os.Symlink(filepath.Join(outside, "Dockerfile"), filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	got, err := bootstrap.Suggest(root, []string{"go"})
	if err != nil || len(got) != 0 {
		t.Fatalf("Suggest = %+v, %v; want no suggestions", got, err)
	}
}

func TestSuggestNamesTheWalkFailure(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "missing")
	got, err := bootstrap.Suggest(root, nil)
	if len(got) != 0 || !errors.Is(err, fs.ErrNotExist) || !strings.HasPrefix(err.Error(), "bootstrap: suggest capabilities: ") {
		t.Fatalf("Suggest = %+v, %v; want the wrapped walk failure", got, err)
	}
}

func TestSuggestNeedsBothRootHexagonalDirectories(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		dirs    []string
		files   []string
		enabled []string
		want    []bootstrap.Suggestion
	}{
		{
			"empty pair",
			[]string{"internal/domain", "internal/application"},
			nil,
			[]string{"go"},
			[]bootstrap.Suggestion{{Capability: "hexagonal", Evidence: "internal/domain"}},
		},
		{"missing application", []string{"internal/domain"}, nil, []string{"go"}, nil},
		{"missing domain", []string{"internal/application"}, nil, []string{"go"}, nil},
		{"domain is a file", []string{"internal/application"}, []string{"internal/domain"}, []string{"go"}, nil},
		{"application is a file", []string{"internal/domain"}, []string{"internal/application"}, []string{"go"}, nil},
		{"nested pair", []string{"nested/internal/domain", "nested/internal/application"}, nil, []string{"go"}, nil},
		{"already enabled", []string{"internal/domain", "internal/application"}, nil, []string{"go", "hexagonal"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			for _, path := range tc.dirs {
				if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(path)), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			for _, path := range tc.files {
				writeBefore(t, filepath.Join(root, filepath.FromSlash(path)), "x\n")
			}
			got, err := bootstrap.Suggest(root, tc.enabled)
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("Suggest = %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}
