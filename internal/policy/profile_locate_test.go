package policy_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// acmeCommand is the install command every refusal of the example pin names.
const acmeCommand = ": run golaw profile install https://example.com/acme.git@0.3.0"

// otherDigest is a well formed digest the example pin does not carry.
var otherDigest = "sha256:" + strings.Repeat("f", 64)

// stampAt is the example stamp at another version and digest.
func stampAt(version, digest string) string {
	text := strings.Replace(acmeStamp, "version: 0.3.0", "version: "+version, 1)
	return strings.Replace(text, "sha256:"+acmeDigest, digest, 1)
}

func TestStoredStampPathSitsBesideTheCurrentBundle(t *testing.T) {
	t.Parallel()
	config := filepath.Join("home", "config")
	want := filepath.Join(config, "profiles", "acme@0.3.0", policy.StampFile)
	if got := policy.StoredStampPath(config, "acme", "0.3.0"); got != want {
		t.Errorf("StoredStampPath = %s, want %s", got, want)
	}
	if filepath.Dir(filepath.Dir(want)) != filepath.Dir(filepath.Dir(policy.StampPath(config, "acme"))) {
		t.Errorf("the store and the current bundle live under one profiles directory")
	}
}

// installed writes the current stamp and the stored stamp at 0.3.0 of the
// example profile under a fresh config directory, each when not empty.
func installed(t *testing.T, current, stored string) string {
	t.Helper()
	config := t.TempDir()
	if current != "" {
		write(t, config, "profiles/acme/profile.yml", current)
	}
	if stored != "" {
		write(t, config, "profiles/acme@0.3.0/profile.yml", stored)
	}
	return config
}

// TestLocateProfileFindsTheCurrentOrTheStoredCopy pins contract D3: the
// current bundle answers when it matches the pin on every field, and the
// stored copy at the pinned version answers when the current one differs
// on version or digest or is absent, so two repositories pinned at two
// versions of one profile both load.
func TestLocateProfileFindsTheCurrentOrTheStoredCopy(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		current string // the current stamp, absent when empty
		stored  string // the stored stamp at 0.3.0, absent when empty
		dir     string // the directory the answer names, under profiles/
		isStore bool
	}{
		{name: "current matches", current: acmeStamp, dir: "acme"},
		{name: "current matches beside a store", current: acmeStamp, stored: acmeStamp, dir: "acme"},
		{name: "current newer, store at the pin", current: stampAt("0.4.0", otherDigest), stored: acmeStamp, dir: "acme@0.3.0", isStore: true},
		{name: "current older, store at the pin", current: stampAt("0.2.0", otherDigest), stored: acmeStamp, dir: "acme@0.3.0", isStore: true},
		{name: "current moved digest, store at the pin", current: stampAt("0.3.0", otherDigest), stored: acmeStamp, dir: "acme@0.3.0", isStore: true},
		{name: "store alone", stored: acmeStamp, dir: "acme@0.3.0", isStore: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			config := installed(t, tc.current, tc.stored)
			got, err := policy.LocateProfile(config, acmeRef())
			if err != nil {
				t.Fatal(err)
			}
			want := filepath.Join(config, "profiles", tc.dir)
			if got.Dir != want || got.Stored != tc.isStore || got.Stamp.Version != "0.3.0" || got.Stamp.Digest != acmeRef().Digest {
				t.Errorf("LocateProfile = %+v, want the bundle at %s (stored %v)", got, want, tc.isStore)
			}
			if s, err := policy.VerifyProfile(config, acmeRef()); err != nil || s != got.Stamp {
				t.Errorf("VerifyProfile = %+v, %v, want the located stamp", s, err)
			}
		})
	}
}

// TestLocateProfileRefusesWithTheInstallCommand pins the A19 texts of D8:
// a missing bundle keeps the M3 text byte for byte, and every field that
// differs names the pinned value, the installed one and the exact command;
// the answer still carries what was found, for doctor to tell the cases.
func TestLocateProfileRefusesWithTheInstallCommand(t *testing.T) {
	t.Parallel()
	pin := "sha256:" + acmeDigest
	cases := []struct {
		name, current, stored, want, foundVersion string
		foundStored                               bool
	}{
		{name: "nothing installed", want: "invalid policy: profile acme is not installed" + acmeCommand},
		{
			name: "current older, no store", current: stampAt("0.2.0", otherDigest), foundVersion: "0.2.0",
			want: "profile acme: version is 0.3.0 in golaw.yml and 0.2.0 in the installed bundle" + acmeCommand,
		},
		{
			name: "current newer, no store", current: stampAt("0.4.0", otherDigest), foundVersion: "0.4.0",
			want: "profile acme: version is 0.3.0 in golaw.yml and 0.4.0 in the installed bundle" + acmeCommand,
		},
		{
			name: "current at the pin with another digest", current: stampAt("0.3.0", otherDigest), foundVersion: "0.3.0",
			want: "profile acme: digest is " + pin + " in golaw.yml and " + otherDigest + " in the installed bundle" + acmeCommand,
		},
		{
			name: "store at the pin with another digest", current: stampAt("0.4.0", otherDigest), stored: stampAt("0.3.0", otherDigest),
			foundVersion: "0.3.0", foundStored: true,
			want: "profile acme: digest is " + pin + " in golaw.yml and " + otherDigest + " in the installed bundle" + acmeCommand,
		},
		{
			name: "current from another source", current: strings.Replace(acmeStamp, "acme.git", "mirror.git", 1), stored: acmeStamp,
			foundVersion: "0.3.0",
			want:         "profile acme: source is https://example.com/acme.git in golaw.yml and https://example.com/mirror.git in the installed bundle" + acmeCommand,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			config := installed(t, tc.current, tc.stored)
			got, err := policy.LocateProfile(config, acmeRef())
			if !errors.Is(err, model.ErrPolicy) || !strings.HasSuffix(err.Error(), tc.want) {
				t.Fatalf("LocateProfile error = %v, want it to end with %q", err, tc.want)
			}
			if got.Stamp.Version != tc.foundVersion || got.Stored != tc.foundStored || (tc.foundVersion == "") != (got.Dir == "") {
				t.Errorf("found = %+v, want version %q stored %v", got, tc.foundVersion, tc.foundStored)
			}
			if _, verr := policy.VerifyProfile(config, acmeRef()); verr == nil || verr.Error() != err.Error() {
				t.Errorf("VerifyProfile = %v, want the error of LocateProfile", verr)
			}
		})
	}
}

// TestLocateProfileKeepsAnUnreadableStampApart pins that a stamp that does
// not parse is its own error, never a missing bundle and never a fallback.
func TestLocateProfileKeepsAnUnreadableStampApart(t *testing.T) {
	t.Parallel()
	broken := t.TempDir()
	write(t, broken, "profiles/acme/profile.yml", "name: [\n")
	write(t, broken, "profiles/acme@0.3.0/profile.yml", acmeStamp)
	if _, err := policy.LocateProfile(broken, acmeRef()); !errors.Is(err, model.ErrPolicy) || strings.Contains(err.Error(), "install") {
		t.Errorf("a broken current stamp: %v", err)
	}
	brokenStore := t.TempDir()
	write(t, brokenStore, "profiles/acme/profile.yml", stampAt("0.4.0", otherDigest))
	write(t, brokenStore, "profiles/acme@0.3.0/profile.yml", "name: [\n")
	if _, err := policy.LocateProfile(brokenStore, acmeRef()); !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), "acme@0.3.0") {
		t.Errorf("a broken stored stamp names its path: %v", err)
	}
}
