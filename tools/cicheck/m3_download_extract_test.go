package cicheck_test

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

type m3Downloaded struct {
	pin                 m3DownloadPin
	line                m3PinLine
	verified, installed bool
}

type m3DownloadReader struct {
	paths map[string]*m3Downloaded
	all   []*m3Downloaded
}

func m3DownloadPins(steps []assets.RunStep) ([]m3DownloadPin, error) {
	reader := m3DownloadReader{paths: make(map[string]*m3Downloaded)}
	for _, line := range m3PinLines(steps) {
		if err := reader.read(line); err != nil {
			return nil, err
		}
	}
	return reader.pins()
}

func (reader *m3DownloadReader) read(line m3PinLine) error {
	fields := strings.Fields(line.Text)
	if len(fields) == 0 {
		return nil
	}
	switch fields[0] {
	case "curl":
		return reader.download(line)
	case "printf":
		return reader.verify(line)
	case "install", "tar":
		return reader.install(line, fields[0])
	default:
		return nil
	}
}

func (reader *m3DownloadReader) download(line m3PinLine) error {
	pattern := `^curl -fsSL https://github\.com/([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)/releases/download/(v[0-9]+\.[0-9]+\.[0-9]+)/([A-Za-z0-9_.-]+) -o "(\$RUNNER_TEMP/[A-Za-z0-9_./-]+)"$`
	match := regexp.MustCompile(pattern).FindStringSubmatch(line.Text)
	if match == nil {
		return line.problem("unrecognized download form")
	}
	pin := m3DownloadPin{Repository: match[1], Tag: match[2], Asset: match[3]}
	if err := m3DownloadChecksumName(&pin, line); err != nil {
		return err
	}
	key := line.Job + "\x00" + match[4]
	if previous := reader.paths[key]; previous != nil && !previous.installed {
		return line.problem("destination overwritten before verification and installation")
	}
	state := &m3Downloaded{pin: pin, line: line}
	reader.paths[key] = state
	reader.all = append(reader.all, state)
	return nil
}

func m3DownloadChecksumName(pin *m3DownloadPin, line m3PinLine) error {
	switch pin.Repository {
	case "hadolint/hadolint":
		if !strings.HasPrefix(pin.Asset, "hadolint-") {
			return line.problem("unrecognized hadolint asset name")
		}
		pin.ChecksumAsset = "checksums.sha256"
	case "aquasecurity/trivy":
		pattern := `^trivy_([0-9]+\.[0-9]+\.[0-9]+)_[A-Za-z0-9_-]+\.tar\.gz$`
		match := regexp.MustCompile(pattern).FindStringSubmatch(pin.Asset)
		if len(match) != 2 || pin.Tag != "v"+match[1] {
			return line.problem("Trivy asset version does not match the URL")
		}
		pin.ChecksumAsset = "trivy_" + match[1] + "_checksums.txt"
	default:
		return line.problem("unrecognized published checksum filename form")
	}
	return nil
}

func (reader *m3DownloadReader) verify(line m3PinLine) error {
	pattern := `^printf '%s  %s\\n' '([a-f0-9]{64})' "(\$RUNNER_TEMP/[A-Za-z0-9_./-]+)" \| sha256sum -c -$`
	match := regexp.MustCompile(pattern).FindStringSubmatch(line.Text)
	if match == nil {
		return line.problem("unrecognized SHA-256 verification form")
	}
	state := reader.paths[line.Job+"\x00"+match[2]]
	if state == nil {
		return line.problem("verification has no preceding download for this path in the same job")
	}
	if state.installed {
		return line.problem("verification follows installation of the download")
	}
	if state.verified && state.pin.SHA256 != match[1] {
		return line.problem("conflicting checksums for the same download")
	}
	state.pin.SHA256 = match[1]
	state.verified = true
	return nil
}

func (reader *m3DownloadReader) install(line m3PinLine, command string) error {
	pattern := `^install -m 0755 "(\$RUNNER_TEMP/[A-Za-z0-9_./-]+)" "\$RUNNER_TEMP/golaw-bin/hadolint"$`
	repository := "hadolint/hadolint"
	if command == "tar" {
		pattern = `^tar -xzf "(\$RUNNER_TEMP/[A-Za-z0-9_./-]+)" -C "\$RUNNER_TEMP/golaw-bin" trivy$`
		repository = "aquasecurity/trivy"
	}
	match := regexp.MustCompile(pattern).FindStringSubmatch(line.Text)
	if match == nil {
		return line.problem("unrecognized downloaded-tool installation form")
	}
	state := reader.paths[line.Job+"\x00"+match[1]]
	if state == nil || state.pin.Repository != repository {
		return line.problem("installation has no matching download in the same job")
	}
	if !state.verified {
		return line.problem("installation precedes SHA-256 verification of the download")
	}
	state.installed = true
	return nil
}

func (reader *m3DownloadReader) pins() ([]m3DownloadPin, error) {
	pins := make([]m3DownloadPin, 0)
	seen := make(map[string]m3DownloadPin)
	for _, state := range reader.all {
		if !state.verified {
			return nil, state.line.problem("download has no SHA-256 verification")
		}
		if !state.installed {
			return nil, state.line.problem("download has no installation")
		}
		key := state.pin.Repository + "\x00" + state.pin.Tag + "\x00" + state.pin.Asset
		if previous, exists := seen[key]; exists {
			if previous != state.pin {
				return nil, state.line.problem("conflicting pins for the same release asset")
			}
			continue
		}
		seen[key] = state.pin
		pins = append(pins, state.pin)
	}
	slices.SortFunc(pins, func(a, b m3DownloadPin) int {
		return cmp.Or(cmp.Compare(a.Repository, b.Repository), cmp.Compare(a.Tag, b.Tag),
			cmp.Compare(a.Asset, b.Asset))
	})
	return pins, nil
}

func TestM3DownloadPinBinding(t *testing.T) {
	t.Parallel()
	commands := m3DownloadCommands(t)
	hadolint, trivy := strings.Join(commands[:3], "\n"), strings.Join(commands[3:], "\n")
	one := func(run string) []assets.RunStep { return []assets.RunStep{{Job: "ci", Run: run}} }
	pair := func(run string) []assets.RunStep {
		return []assets.RunStep{{Job: "ci", Run: hadolint}, {Job: "other", Run: run}}
	}
	url := strings.Split(strings.Fields(commands[3])[2], "/")
	wrongVersion := strings.Replace(trivy, "/"+url[len(url)-2]+"/", "/v9999.0.0/", 1)
	wrongDigest := regexp.MustCompile(`[a-f0-9]{64}`).ReplaceAllString(hadolint, strings.Repeat("a", 64))
	for _, test := range []struct {
		name  string
		steps []assets.RunStep
		fail  bool
	}{
		{"matching steps", []assets.RunStep{{Job: "ci", Run: commands[0]}, {Job: "ci", Run: commands[1]}, {Job: "ci", Run: commands[2]}}, false},
		{"different job", []assets.RunStep{{Job: "ci", Run: commands[0]}, {Job: "other", Run: strings.Join(commands[1:3], "\n")}}, true},
		{"missing installation", one(strings.Join(commands[:2], "\n")), true},
		{"missing verification", one(commands[0] + "\n" + commands[2]), true},
		{"tar before verification", one(commands[3] + "\n" + commands[5] + "\n" + commands[4]), true},
		{"unknown release", one(strings.Replace(hadolint, "hadolint/hadolint/", "other/tool/", 1)), true},
		{"unknown curl syntax", one(strings.Replace(hadolint, "curl -fsSL", "curl -L", 1)), true},
		{"unknown install syntax", one(strings.Replace(hadolint, "install -m 0755", "install -m 0700", 1)), true},
		{"URL asset mismatch", one(wrongVersion), true},
		{"conflicting job pins", pair(wrongDigest), true},
		{"identical job pins", pair(hadolint), false},
		{"identical repeated pins", one(hadolint + "\n" + hadolint), false},
		{"uninstalled path overwrite", one(commands[0] + "\n" + hadolint), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			pins, err := m3DownloadPins(test.steps)
			if (err != nil) != test.fail {
				t.Fatalf("pins=%+v, error=%v", pins, err)
			}
			if !test.fail && len(pins) != 1 {
				t.Fatalf("unique pins=%+v", pins)
			}
			if test.fail && (!strings.Contains(err.Error(), "job ") ||
				!strings.Contains(err.Error(), "run step ") || !strings.Contains(err.Error(), "line ")) {
				t.Fatalf("extraction error has no command context: %v", err)
			}
		})
	}
}

func m3DownloadCommands(t *testing.T) []string {
	t.Helper()
	steps, err := assets.RunSteps(m3CI(t, []string{"image"}))
	if err != nil {
		t.Fatal(err)
	}
	var commands []string
	for _, line := range m3PinLines(steps) {
		if line.Job == "ci" && slices.Contains([]string{"curl", "printf", "install", "tar"}, strings.Fields(line.Text)[0]) {
			commands = append(commands, line.Text)
		}
	}
	if len(commands) != 6 {
		t.Fatalf("download commands=%v", commands)
	}
	return commands
}
