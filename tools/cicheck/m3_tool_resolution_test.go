package cicheck_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

var (
	errM3ReleaseUnavailable = errors.New("release unavailable")
	errM3UnexpectedNPM      = errors.New("unexpected npm resolution")
	errM3VersionUnavailable = errors.New("version unavailable")
	errM3UnexpectedRelease  = errors.New("unexpected release resolution")
)

func TestM3PublishedDownloadChecksums(t *testing.T) {
	t.Parallel()
	pin := m3DownloadPin{
		Repository: "hadolint/hadolint", Tag: "v2.14.0",
		Asset:         "hadolint-linux-x86_64",
		ChecksumAsset: "hadolint-linux-x86_64.sha256",
		SHA256:        "6bf226944684f56c84dd014e8b979d27425c0148f61b3bd99bcc6f39e9dc5a47",
	}
	for _, test := range []struct {
		name, checksum string
		assets         []string
		err            error
		fail           bool
	}{
		{
			"binary marker", pin.SHA256 + " *" + pin.Asset + "\n",
			[]string{pin.Asset, pin.ChecksumAsset},
			nil, false,
		},
		{
			"text marker", pin.SHA256 + "  " + pin.Asset + "\n",
			[]string{pin.Asset, pin.ChecksumAsset},
			nil, false,
		},
		{
			"wrong digest", strings.Repeat("a", 64) + "  " + pin.Asset + "\n",
			[]string{pin.Asset, pin.ChecksumAsset},
			nil, true,
		},
		{
			"wrong file", pin.SHA256 + "  other\n",
			[]string{pin.Asset, pin.ChecksumAsset},
			nil, true,
		},
		{
			"missing executable", pin.SHA256 + "  " + pin.Asset + "\n",
			[]string{pin.ChecksumAsset},
			nil, true,
		},
		{
			"missing checksum asset", pin.SHA256 + "  " + pin.Asset + "\n",
			[]string{pin.Asset},
			nil, true,
		},
		{"unavailable release", "", nil, errM3ReleaseUnavailable, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			err := m3CheckInstalledPins(
				m3InstalledPins{Downloads: []m3DownloadPin{pin}},
				func(got m3DownloadPin) ([]string, []byte, error) {
					calls++
					if got != pin {
						t.Fatalf("pin=%+v", got)
					}
					return test.assets, []byte(test.checksum), test.err
				},
				func(name, version string) (string, error) {
					t.Fatalf("unexpected npm resolution: %s@%s", name, version)
					return "", errM3UnexpectedNPM
				},
			)
			if calls != 1 || (err != nil) != test.fail {
				t.Fatalf("calls=%d, error=%v", calls, err)
			}
			if test.fail && (!strings.Contains(err.Error(), pin.Repository) ||
				!strings.Contains(err.Error(), pin.Tag) ||
				!strings.Contains(err.Error(), pin.Asset)) {
				t.Fatalf("error does not name the pin: %v", err)
			}
		})
	}
}

func TestM3NPMVersionsResolveExactly(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, answer string
		err          error
		fail         bool
	}{
		{"near", "2.35.0\n", nil, false},
		{"wrong version", "2.34.0\n", nil, true},
		{"empty", "", nil, true},
		{"missing", "", errM3VersionUnavailable, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := m3CheckInstalledPins(
				m3InstalledPins{NPM: map[string]string{"squawk-cli": "2.35.0"}},
				func(pin m3DownloadPin) ([]string, []byte, error) {
					t.Fatalf("unexpected release resolution: %+v", pin)
					return nil, nil, errM3UnexpectedRelease
				},
				func(name, version string) (string, error) {
					if name != "squawk-cli" || version != "2.35.0" {
						t.Fatalf("npm pin=%s@%s", name, version)
					}
					return test.answer, test.err
				},
			)
			if (err != nil) != test.fail {
				t.Fatalf("error=%v, want failure=%t", err, test.fail)
			}
			if test.fail && !strings.Contains(err.Error(), "squawk-cli@2.35.0") {
				t.Fatalf("error does not name the npm pin: %v", err)
			}
		})
	}
}

func TestM3PublishedChecksumBoundaries(t *testing.T) {
	t.Parallel()
	pin := m3DownloadPin{
		Repository: "example/tool", Tag: "v1.0.0", Asset: "tool.tar.gz",
		ChecksumAsset: "checksums.txt", SHA256: strings.Repeat("a", 64),
	}
	valid := pin.SHA256 + "  " + pin.Asset + "\n"
	for _, test := range []struct {
		name, data string
		fail       bool
	}{
		{"identical entries", valid + valid, false},
		{"uppercase hex", strings.ToUpper(pin.SHA256) + " *" + pin.Asset + "\n", false},
		{"CRLF", strings.ReplaceAll(valid, "\n", "\r\n"), false},
		{"other asset", pin.SHA256 + "  other.tar.gz\n" + valid, false},
		{"malformed neighboring asset", valid + "abc  other-" + pin.Asset + "\n", false},
		{"conflicting entries", valid + strings.Repeat("b", 64) + "  " + pin.Asset + "\n", true},
		{"short digest", "abc  " + pin.Asset + "\n", true},
		{"bad hex", strings.Repeat("z", 64) + "  " + pin.Asset + "\n", true},
		{"missing marker", pin.SHA256 + " " + pin.Asset + "\n", true},
		{"wrong marker", pin.SHA256 + " -" + pin.Asset + "\n", true},
		{"malformed duplicate", valid + "abc  " + pin.Asset + "\n", true},
		{"filename prefix", pin.SHA256 + "  other-" + pin.Asset + "\n", true},
		{"filename suffix", pin.SHA256 + "  " + pin.Asset + ".sig\n", true},
		{"empty checksum", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := m3PublishedChecksum(pin, []byte(test.data))
			if (err != nil) != test.fail {
				t.Fatalf("error=%v, want failure=%t", err, test.fail)
			}
			if err != nil && !strings.Contains(err.Error(), m3ReleasePinLabel(pin)) {
				t.Fatalf("error does not name the complete pin: %v", err)
			}
		})
	}
}

func TestM3InstalledResolutionOrderAndEmptyInventory(t *testing.T) {
	t.Parallel()
	digest := strings.Repeat("a", 64)
	pins := m3InstalledPins{
		Downloads: []m3DownloadPin{
			{Repository: "z/tool", Tag: "v1.0.0", Asset: "tool", ChecksumAsset: "sum", SHA256: digest},
			{Repository: "a/tool", Tag: "v1.0.0", Asset: "tool", ChecksumAsset: "sum", SHA256: digest},
		},
		NPM: map[string]string{"z-package": "2.0.0", "a-package": "1.0.0"},
	}
	original := slices.Clone(pins.Downloads)
	var calls []string
	release := func(pin m3DownloadPin) ([]string, []byte, error) {
		calls = append(calls, pin.Repository)
		return []string{pin.Asset, pin.ChecksumAsset}, []byte(digest + "  " + pin.Asset), nil
	}
	npm := func(name, version string) (string, error) {
		calls = append(calls, name)
		return version, nil
	}
	if err := m3CheckInstalledPins(pins, release, npm); err != nil {
		t.Fatal(err)
	}
	want := []string{"a/tool", "z/tool", "a-package", "z-package"}
	if !slices.Equal(calls, want) || !slices.Equal(pins.Downloads, original) {
		t.Fatalf("calls=%v, want %v; downloads=%v, want unchanged %v", calls, want, pins.Downloads, original)
	}
	calls = nil
	if err := m3CheckInstalledPins(m3InstalledPins{Go: map[string]string{"example.test/tool": "v1.0.0"}}, release, npm); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("empty external inventory resolved %v", calls)
	}
}
