// Package install keeps installation ownership outside adopting repositories.
package install

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// ProjectIdentity prevents path aliases from creating separate enrollments.
func ProjectIdentity(root string) (string, string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve root: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", "", fmt.Errorf("resolve root links: %w", err)
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", "", fmt.Errorf("inspect root: %w", err)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("repository root: %w", ErrPath)
	}
	return canonical, Digest([]byte(canonical)), nil
}

// Digest binds ownership records to exact bytes.
func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
