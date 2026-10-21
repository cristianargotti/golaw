//go:build network

package cicheck_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/runner"
)

func m3NetworkRelease(t *testing.T, pin m3DownloadPin) ([]string, []byte, error) {
	t.Helper()
	label := m3ReleasePinLabel(pin)
	endpoint := "repos/" + pin.Repository + "/releases/tags/" + pin.Tag
	data, err := m3NetworkCommand(t, "gh", "api", endpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve release %s: %w", label, err)
	}
	var metadata struct {
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if parseErr := json.Unmarshal(data, &metadata); parseErr != nil {
		return nil, nil, fmt.Errorf("decode release %s: %w", label, parseErr)
	}
	names := make([]string, 0, len(metadata.Assets))
	checksumURL := ""
	for _, asset := range metadata.Assets {
		names = append(names, asset.Name)
		if asset.Name == pin.ChecksumAsset {
			checksumURL = asset.URL
		}
	}
	if !slices.Contains(names, pin.Asset) || checksumURL == "" {
		return nil, nil, fmt.Errorf("%w: release %s must publish its executable and %s URL", errM3InstalledPin, label, pin.ChecksumAsset)
	}
	checksum, err := m3NetworkCommand(t, "curl", "-fsSL", checksumURL)
	if err != nil {
		return nil, nil, fmt.Errorf("download published checksum for %s: %w", label, err)
	}
	return names, checksum, nil
}

func m3NetworkNPM(t *testing.T, name, version string) (string, error) {
	t.Helper()
	pin := name + "@" + version
	output, err := m3NetworkCommand(t, "npm", "view", pin, "version")
	if err != nil {
		return "", fmt.Errorf("resolve npm %s: %w", pin, err)
	}
	return string(output), nil
}

func m3NetworkCommand(t *testing.T, name string, args ...string) ([]byte, error) {
	t.Helper()
	command := exec.CommandContext(t.Context(), name, args...)
	stdout, stderr := &m3PinOutput{}, &m3PinOutput{}
	command.Stdout, command.Stderr, command.WaitDelay = stdout, stderr, time.Second
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("run %s: %w: %s", name, err, strings.TrimSpace(stderr.data.String()))
	}
	if stdout.overflow || stderr.overflow {
		return nil, fmt.Errorf("%w: %s output exceeds %d bytes", errM3InstalledPin, name, runner.MaxOutputBytes)
	}
	return stdout.data.Bytes(), nil
}

type m3PinOutput struct {
	data     bytes.Buffer
	overflow bool
}

func (output *m3PinOutput) Write(data []byte) (int, error) {
	room := runner.MaxOutputBytes - output.data.Len()
	if len(data) > room {
		output.overflow = true
	}
	if _, err := output.data.Write(data[:min(room, len(data))]); err != nil {
		return 0, fmt.Errorf("capture pin resolver output: %w", err)
	}
	return len(data), nil
}
