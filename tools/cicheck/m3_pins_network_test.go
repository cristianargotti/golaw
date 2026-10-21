//go:build network

package cicheck_test

import (
	"maps"
	"slices"
	"testing"
)

func TestM3InstalledPinsResolve(t *testing.T) {
	t.Parallel()
	all := m3InstalledPins{
		Go: make(map[string]string), NPM: make(map[string]string),
		Downloads: []m3DownloadPin{},
	}
	downloads := make(map[string]m3DownloadPin)
	documents := m3PinDocuments(t)
	for _, name := range slices.Sorted(maps.Keys(documents)) {
		pins, err := m3ToolPins(documents[name])
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		m3MergeInstalledPins(t, all, downloads, pins)
	}
	for _, key := range slices.Sorted(maps.Keys(downloads)) {
		all.Downloads = append(all.Downloads, downloads[key])
	}
	if len(all.Downloads) != 2 || len(all.NPM) != 2 {
		t.Fatalf("incomplete installed-tool inventory: %+v", all)
	}
	err := m3CheckInstalledPins(
		all,
		func(pin m3DownloadPin) ([]string, []byte, error) {
			return m3NetworkRelease(t, pin)
		},
		func(name, version string) (string, error) {
			return m3NetworkNPM(t, name, version)
		},
	)
	if err != nil {
		t.Fatal(err)
	}
}

func m3MergeInstalledPins(t *testing.T, all m3InstalledPins, downloads map[string]m3DownloadPin, pins m3InstalledPins) {
	t.Helper()
	for module, version := range pins.Go {
		if previous, ok := all.Go[module]; ok && previous != version {
			t.Fatalf("conflicting Go pin: %s", module)
		}
		all.Go[module] = version
	}
	for pkg, version := range pins.NPM {
		if previous, ok := all.NPM[pkg]; ok && previous != version {
			t.Fatalf("conflicting npm pin: %s", pkg)
		}
		all.NPM[pkg] = version
	}
	for _, pin := range pins.Downloads {
		key := pin.Repository + "@" + pin.Tag + "/" + pin.Asset
		if previous, ok := downloads[key]; ok && previous != pin {
			t.Fatalf("conflicting download pin: %s", key)
		}
		downloads[key] = pin
	}
}
