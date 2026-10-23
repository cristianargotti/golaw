package profile_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// fixtureSource is the local bare source of the fixture at its tag.
func fixtureSource(t *testing.T) profile.Source {
	t.Helper()
	url, tag := profiletest.BareSource(t)
	src, err := profile.ParseSource(url + "@" + tag)
	if err != nil {
		t.Fatal(err)
	}
	return src
}

// leftovers lists what a fetch left in the profiles directory.
func leftovers(t *testing.T, config string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(profile.Dir(config, profiletest.Name)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestFetchClonesTheTagUnderTheProfilesDirectory(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	dir, err := profile.Fetch(t.Context(), config, fixtureSource(t), gittest.Isolated())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(dir) != filepath.Dir(profile.Dir(config, profiletest.Name)) || !strings.HasPrefix(filepath.Base(dir), ".fetch-") {
		t.Fatalf("the clone is not a hidden directory of the profiles directory: %s", dir)
	}
	b, err := profile.Read(dir)
	if err != nil || b.Stamp.Digest != profiletest.Ref(t).Digest {
		t.Fatalf("the clone reads as the fixture: %+v, %v", b, err)
	}
}

// TestFetchClonesTheBareVersionOfAVTag pins the command a refusal prints:
// it names the bare version of the pin (D8), and a source tagged v1.2.3
// still serves it, because D9 reads both forms as one version.
func TestFetchClonesTheBareVersionOfAVTag(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	src := profile.Source{URL: fixtureSource(t).URL, Tag: "1.2.3", Version: "1.2.3"}
	dir, err := profile.Fetch(t.Context(), config, src, gittest.Isolated())
	if err != nil {
		t.Fatal(err)
	}
	if b, err := profile.Read(dir); err != nil || b.Stamp.Version != "1.2.3" {
		t.Fatalf("the clone reads as version 1.2.3: %+v, %v", b, err)
	}
	if left := leftovers(t, config); len(left) != 1 {
		t.Fatalf("the clone of the other form leaves only itself: %v", left)
	}
}

// absentCause is what git names for a clone of a local path that does not
// exist: git clone reads such a path as a URL when it holds a colon, a
// Windows drive letter included, and then its transport names the path.
func absentCause(path string) string {
	if strings.Contains(path, ":") {
		return "does not appear to be a git repository"
	}
	return "does not exist"
}

// TestFetchRefusesWhatItCannotClone names the cause git gives on one line,
// the line that names it when git adds a hint of several lines after it,
// as it does for a source it reaches through a transport.
func TestFetchRefusesWhatItCannotClone(t *testing.T) {
	t.Parallel()
	src := fixtureSource(t)
	absent := filepath.Join(t.TempDir(), "absent.git")
	cases := map[string]struct {
		src  profile.Source
		want string
	}{
		"an absent source":   {profile.Source{URL: absent, Tag: "v1.2.3", Version: "1.2.3"}, absentCause(absent)},
		"an absent tag":      {profile.Source{URL: src.URL, Tag: "v9.9.9", Version: "9.9.9"}, "v9.9.9"},
		"an option as a url": {profile.Source{URL: "--upload-pack=touch", Tag: "v1.2.3", Version: "1.2.3"}, "--upload-pack=touch"},
	}
	if filepath.VolumeName(absent) == "" {
		// Git for Windows reads a colon past the drive letter as no local
		// path, so the transport answer is held there by the case above.
		colon := filepath.Join(t.TempDir(), "a:b", "absent.git")
		cases["an absent source whose path holds a colon"] = struct {
			src  profile.Source
			want string
		}{profile.Source{URL: colon, Tag: "v1.2.3", Version: "1.2.3"}, absentCause(colon)}
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			config := t.TempDir()
			_, err := profile.Fetch(t.Context(), config, tc.src, gittest.Isolated())
			if !errors.Is(err, profile.ErrBundle) || !errors.Is(err, model.ErrPolicy) {
				t.Fatalf("Fetch = %v", err)
			}
			if msg := err.Error(); !strings.Contains(msg, "clone "+tc.src.URL+" at "+tc.src.Tag+": ") || !strings.Contains(msg, tc.want) || strings.Contains(msg, "\n") {
				t.Fatalf("the error names the URL, the tag and git's cause on one line: %q", msg)
			}
			if left := leftovers(t, config); len(left) != 0 {
				t.Fatalf("a failed fetch left %v", left)
			}
		})
	}
}

func TestFetchStopsWithItsContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	config := t.TempDir()
	if _, err := profile.Fetch(ctx, config, fixtureSource(t), gittest.Isolated()); !errors.Is(err, profile.ErrBundle) || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("a cancelled fetch = %v", err)
	}
	if left := leftovers(t, config); len(left) != 0 {
		t.Fatalf("a cancelled fetch left %v", left)
	}
}

func TestFetchNamesAConfigDirectoryItCannotWrite(t *testing.T) {
	t.Parallel()
	config := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(config, []byte("a file where the config directory goes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Fetch(t.Context(), config, fixtureSource(t), gittest.Isolated()); !errors.Is(err, profile.ErrBundle) || !strings.Contains(err.Error(), "create ") {
		t.Fatalf("Fetch over a file = %v", err)
	}
}
