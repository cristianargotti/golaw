package setup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/install"
)

func validateReceipt(receipt *Receipt) error {
	if receipt.Schema != ReceiptSchema {
		return fmt.Errorf("%w: unsupported schema %d; inspect the receipt before running golaw setup again",
			ErrReceipt, receipt.Schema)
	}
	if strings.TrimSpace(receipt.Identity) == "" {
		return fmt.Errorf("%w: identity is empty", ErrReceipt)
	}
	seen := make(map[string]bool)
	for _, tool := range receipt.Tools {
		if seen[tool.Name] {
			return fmt.Errorf("%w: duplicate tool %q", ErrReceipt, tool.Name)
		}
		seen[tool.Name] = true
	}
	return validateReceiptHooks(receipt.HooksPath, receipt.Hooks)
}

func validateReceiptHooks(directory string, hooks []install.Artifact) error {
	seen := make(map[string]bool)
	for _, hook := range hooks {
		if seen[hook.Path] {
			return fmt.Errorf("%w: duplicate hook %q", ErrReceipt, hook.Path)
		}
		seen[hook.Path] = true
		if err := validateReceiptHook(directory, hook); err != nil {
			return err
		}
	}
	return nil
}

func validateReceiptHook(directory string, hook install.Artifact) error {
	parent := filepath.Clean(directory)
	if !filepath.IsAbs(directory) || hook.Path != filepath.Clean(hook.Path) ||
		hook.Path == parent || filepath.Dir(hook.Path) != parent {
		return fmt.Errorf("%w: hook %q must be a direct child of %q", ErrReceipt, hook.Path, directory)
	}
	digest, err := hex.DecodeString(hook.SHA256)
	if err != nil || len(digest) != sha256.Size {
		return fmt.Errorf("%w: hook %q has a malformed SHA256 hash", ErrReceipt, hook.Path)
	}
	return nil
}

func receiptTarget(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect receipt target: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: target is not a regular file", ErrReceipt)
	}
	return nil
}

// Typed decoding runs first, bounding the nesting to the receipt's own fields.
func validateJSONKeys(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%w: read JSON value: %w", ErrReceipt, err)
	}
	switch token {
	case json.Delim('{'):
		return validateJSONObject(decoder)
	case json.Delim('['):
		for decoder.More() {
			if err := validateJSONKeys(decoder); err != nil {
				return err
			}
		}
		return closeJSONObject(decoder)
	}
	return nil
}

func validateJSONObject(decoder *json.Decoder) error {
	var seen []string
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("%w: read JSON key: %w", ErrReceipt, err)
		}
		key, ok := token.(string)
		if !ok || slices.ContainsFunc(seen, func(other string) bool { return strings.EqualFold(key, other) }) {
			return fmt.Errorf("%w: duplicate JSON key %q", ErrReceipt, key)
		}
		seen = append(seen, key)
		if err := validateJSONKeys(decoder); err != nil {
			return err
		}
	}
	return closeJSONObject(decoder)
}

func closeJSONObject(decoder *json.Decoder) error {
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("%w: close JSON value: %w", ErrReceipt, err)
	}
	return nil
}
