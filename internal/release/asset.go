package release

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// AssetName is the name of the release binary of a platform, as the
// archive template of the lane writes it: golaw_<version>_<goos>-<goarch>,
// with the .exe suffix on windows.
func AssetName(version, goos, goarch string) string {
	name := fmt.Sprintf("golaw_%s_%s-%s", version, goos, goarch)
	if goos == "windows" {
		return name + ".exe"
	}
	return name
}

// FileDigest is the hex SHA-256 of a file, the running executable among
// them; a file that cannot be read is ErrInput naming it.
func FileDigest(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // the path is the executable or a file of the release directory
	if err != nil {
		return "", fmt.Errorf("%w: read %s: %w", ErrInput, path, err)
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return "", fmt.Errorf("%w: read %s: %w", ErrInput, path, err)
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}
