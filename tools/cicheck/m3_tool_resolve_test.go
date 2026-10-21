package cicheck_test

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

var errM3InstalledPin = errors.New("invalid installed tool pin")

func m3CheckInstalledPins(
	pins m3InstalledPins,
	release func(m3DownloadPin) ([]string, []byte, error),
	npm func(string, string) (string, error),
) error {
	downloads := slices.Clone(pins.Downloads)
	slices.SortFunc(downloads, func(a, b m3DownloadPin) int {
		return cmp.Or(cmp.Compare(a.Repository, b.Repository), cmp.Compare(a.Tag, b.Tag), cmp.Compare(a.Asset, b.Asset))
	})
	for _, pin := range downloads {
		assets, checksums, err := release(pin)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", m3ReleasePinLabel(pin), err)
		}
		if !slices.Contains(assets, pin.Asset) || !slices.Contains(assets, pin.ChecksumAsset) {
			return fmt.Errorf("%w: %s must publish both the executable and %s", errM3InstalledPin, m3ReleasePinLabel(pin), pin.ChecksumAsset)
		}
		if err := m3PublishedChecksum(pin, checksums); err != nil {
			return err
		}
	}
	for _, name := range slices.Sorted(maps.Keys(pins.NPM)) {
		version := pins.NPM[name]
		answer, err := npm(name, version)
		if err != nil {
			return fmt.Errorf("resolve npm %s@%s: %w", name, version, err)
		}
		if strings.TrimSpace(answer) != version {
			return fmt.Errorf("%w: npm %s@%s resolved to %q", errM3InstalledPin, name, version, strings.TrimSpace(answer))
		}
	}
	return nil
}

func m3PublishedChecksum(pin m3DownloadPin, data []byte) error {
	linePattern := regexp.MustCompile(`^([0-9a-fA-F]{64}) [ *](.+)$`)
	digest := ""
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		parts := linePattern.FindStringSubmatch(line)
		if parts == nil {
			fields := strings.Fields(line)
			if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == pin.Asset {
				return fmt.Errorf("%w: %s has a malformed published checksum entry", errM3InstalledPin, m3ReleasePinLabel(pin))
			}
			continue
		}
		if parts[2] != pin.Asset {
			continue
		}
		current := strings.ToLower(parts[1])
		if digest != "" && digest != current {
			return fmt.Errorf("%w: %s has conflicting published checksum entries", errM3InstalledPin, m3ReleasePinLabel(pin))
		}
		digest = current
	}
	if digest == "" || digest != pin.SHA256 {
		return fmt.Errorf("%w: %s expects SHA-256 %s, published %q", errM3InstalledPin, m3ReleasePinLabel(pin), pin.SHA256, digest)
	}
	return nil
}

func m3ReleasePinLabel(pin m3DownloadPin) string {
	return pin.Repository + "@" + pin.Tag + "/" + pin.Asset
}
