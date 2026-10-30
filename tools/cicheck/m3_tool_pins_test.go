package cicheck_test

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

func TestM3ToolPinsCoverEveryRendering(t *testing.T) {
	t.Parallel()
	for _, caps := range m3Capabilities() {
		t.Run(strings.Join(caps, ","), func(t *testing.T) {
			t.Parallel()
			got, err := m3ToolPins(m3CI(t, caps))
			if err != nil {
				t.Fatal(err)
			}
			wantGo, wantNPM, wantDownloads := m3ExpectedToolPins(caps)
			if !maps.Equal(got.Go, wantGo) || !maps.Equal(got.NPM, wantNPM) ||
				!reflect.DeepEqual(got.Downloads, wantDownloads) {
				t.Fatalf("pins=%+v, want Go=%v npm=%v downloads=%v",
					got, wantGo, wantNPM, wantDownloads)
			}
		})
	}
}

func m3ExpectedToolPins(caps []string) (map[string]string, map[string]string, []m3DownloadPin) {
	wantGo := map[string]string{"github.com/cristianargotti/golaw/cmd/golaw": "v0.2.0"}
	if slices.Contains(caps, "go") {
		for _, tool := range runner.Toolchain() {
			if tool.Name != "lefthook" {
				wantGo[tool.Module] = "v" + tool.Version
			}
		}
	}
	wantNPM := make(map[string]string)
	if slices.Contains(caps, "api") {
		wantNPM["@redocly/cli"] = "2.5.0"
	}
	if slices.Contains(caps, "db") {
		wantNPM["squawk-cli"] = "2.35.0"
	}
	wantDownloads := []m3DownloadPin{}
	if slices.Contains(caps, "image") {
		wantDownloads = []m3DownloadPin{
			{
				Repository: "aquasecurity/trivy", Tag: "v0.75.0",
				Asset:         "trivy_0.75.0_Linux-64bit.tar.gz",
				ChecksumAsset: "trivy_0.75.0_checksums.txt",
				SHA256:        "c6e65abddb348e25f10549df887045629cf28cc72453cd1c63acb717316b3f3f",
			},
			{
				Repository: "hadolint/hadolint", Tag: "v2.15.1",
				Asset:         "hadolint-linux-x86_64",
				ChecksumAsset: "checksums.sha256",
				SHA256:        "c7187db94eeeeca956519a6af171adc31453941a1e777961f6e680f697c8c507",
			},
		}
	}
	return wantGo, wantNPM, wantDownloads
}

func TestM3ToolPinExtractionBoundaries(t *testing.T) {
	t.Parallel()
	source := string(m3CI(t, []string{"api", "db", "go", "image"}))
	const digest = "c7187db94eeeeca956519a6af171adc31453941a1e777961f6e680f697c8c507"
	const verify = `printf '%s  %s\n' '` + digest +
		`' "$RUNNER_TEMP/hadolint" | sha256sum -c -`
	const install = `install -m 0755 "$RUNNER_TEMP/hadolint" "$RUNNER_TEMP/golaw-bin/hadolint"`
	for _, test := range []struct {
		name, old, next string
		fail            bool
	}{
		{"near", verify, verify, false},
		{"floating npm", "squawk-cli@2.35.0", "squawk-cli@latest", true},
		{"npm range", "@redocly/cli@2.5.0", "@redocly/cli@^2.5.0", true},
		{"floating Go", "cmd/golaw@v0.2.0", "cmd/golaw@latest", true},
		{"short digest", digest, "c7187d", true},
		{"unchecked download", "sha256sum -c -", "cat", true},
		{
			"wrong checked file", verify,
			strings.ReplaceAll(verify, `"$RUNNER_TEMP/hadolint"`, `"$RUNNER_TEMP/other"`),
			true,
		},
		{
			"install before verification",
			verify + "\n          " + install,
			install + "\n          " + verify,
			true,
		},
		{"repeated identical npm", "squawk-cli@2.35.0", "squawk-cli@2.35.0", false},
		{"broken YAML", source, "jobs: [\n", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			data := strings.Replace(source, test.old, test.next, 1)
			if test.old != test.next && data == source {
				t.Fatal("tool-pin mutation did not change its source")
			}
			got, err := m3ToolPins([]byte(data))
			if (err != nil) != test.fail {
				t.Fatalf("pins=%+v, error=%v", got, err)
			}
			if test.fail && strings.TrimSpace(err.Error()) == "" {
				t.Fatal("pin error has no reason")
			}
		})
	}
}

func TestM3InstallExtractionForms(t *testing.T) {
	t.Parallel()
	const goPin = "go install example.test/tool@v1.2.3"
	const npmPin = "npm install --global @example/tool@1.2.3"
	for _, test := range []struct {
		name, command string
		fail          bool
	}{
		{"Go", goPin, false},
		{"scoped npm", npmPin, false},
		{"identical Go", goPin + "\n" + goPin, false},
		{"identical npm", npmPin + "\n" + npmPin, false},
		{"conflicting Go", goPin + "\n" + strings.ReplaceAll(goPin, "v1.2.3", "v1.2.4"), true},
		{"conflicting npm", npmPin + "\n" + strings.ReplaceAll(npmPin, "1.2.3", "1.2.4"), true},
		{"Go flag", "go install -race example.test/tool@v1.2.3", true},
		{"Go flag without module", "go install -race", true},
		{"Go unversioned", "go install example.test/tool", true},
		{"Go branch", "go install example.test/tool@main", true},
		{"Go prefix absent", "go install example.test/tool@1.2.3", true},
		{"Go compound", goPin + " && go version", true},
		{"npm unversioned", "npm install --global @example/tool", true},
		{"npm without name", "npm install --global @1.2.3", true},
		{"npm local", "npm install @example/tool@1.2.3", true},
		{"npm extra flag", npmPin + " --force", true},
		{"npm compound", npmPin + " || true", true},
		{"unknown installer", "wget https://example.test/tool", true},
		{"empty commands", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			data := "jobs:\n  ci:\n    steps:\n      - run: |\n          " +
				strings.ReplaceAll(test.command, "\n", "\n          ") + "\n"
			if test.command == "" {
				data = "jobs:\n  ci:\n    steps: []\n"
			}
			got, err := m3ToolPins([]byte(data))
			requireM3ExtractionResult(t, got, err, test.fail)
		})
	}
}

func requireM3ExtractionResult(t *testing.T, got m3InstalledPins, err error, fail bool) {
	t.Helper()
	if (err != nil) != fail {
		t.Fatalf("pins=%+v, error=%v", got, err)
	}
	if fail {
		for _, location := range []string{"job ci", "run step 1", "line "} {
			if !strings.Contains(err.Error(), location) {
				t.Fatalf("error omits %q: %v", location, err)
			}
		}
	} else if got.Go == nil || got.NPM == nil || got.Downloads == nil {
		t.Fatalf("unallocated pin inventory: %+v", got)
	}
}

func TestM3GitleaksLinkerPin(t *testing.T) {
	t.Parallel()
	source := string(m3CI(t, []string{"go"}))
	const flag = "-ldflags=-X=github.com/zricethezav/gitleaks/v8/version.Version=v8.30.1"
	for _, replacement := range []string{
		strings.ReplaceAll(flag, "v8.30.1", "v8.30.2"),
		strings.ReplaceAll(flag, "version.Version", "version.Other"),
	} {
		t.Run(replacement, func(t *testing.T) {
			t.Parallel()
			data := strings.Replace(source, flag, replacement, 1)
			if data == source {
				t.Fatal("linker mutation did not change the rendering")
			}
			if _, err := m3ToolPins([]byte(data)); err == nil {
				t.Fatal("mismatched linker pin accepted")
			}
		})
	}
}
