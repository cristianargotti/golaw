package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// projectShare rewrites the installation of a row into the layout a
// release before settings.local.json left: the entries in the shared
// settings.json beside a key of the person, a receipt that names that file
// and no exclude line.
func projectShare(t m4Target) error {
	receipt, err := projectReceipt(t.home)
	if err != nil {
		return err
	}
	local := filepath.Join(t.repo, claudeConfigDir, localSettings)
	shared := filepath.Join(t.repo, claudeConfigDir, sharedSettings)
	var document map[string]json.RawMessage
	body, err := readFile(local)
	if err == nil {
		err = json.Unmarshal(body, &document)
	}
	if err != nil {
		return fmt.Errorf("read the local settings: %w", err)
	}
	document[projectPersonKey] = json.RawMessage(`"fixed"`)
	merged, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf(encodeFailed, shared, err)
	}
	if err := writeFile(shared, merged, fileMode); err != nil {
		return err
	}
	if err := os.Remove(local); err != nil {
		return fmt.Errorf(removeFailed, local, err)
	}
	if err := projectOldReceipt(receipt, local, shared, merged); err != nil {
		return err
	}
	return projectUnexclude(t.repo)
}

// projectReceipt is the one Claude project receipt under a home.
func projectReceipt(home string) (string, error) {
	found, err := filepath.Glob(filepath.Join(home, filepath.FromSlash(projectStatePaths)))
	if err != nil || len(found) != 1 {
		return "", fmt.Errorf("find the one project receipt under %s: %d found: %w", home, len(found), ErrAcceptance)
	}
	return found[0], nil
}

// projectOldReceipt points the receipt at the shared file with the digest
// of its bytes and drops the exclude record no earlier release wrote.
func projectOldReceipt(path, local, shared string, merged []byte) error {
	var receipt map[string]json.RawMessage
	body, err := readFile(path)
	if err == nil {
		err = json.Unmarshal(body, &receipt)
	}
	var artifacts []map[string]string
	if err == nil {
		err = json.Unmarshal(receipt["artifacts"], &artifacts)
	}
	if err != nil {
		return fmt.Errorf("read the project receipt: %w", err)
	}
	sum := sha256.Sum256(merged)
	for _, artifact := range artifacts {
		if artifact["path"] == local {
			artifact["path"], artifact["sha256"] = shared, hex.EncodeToString(sum[:])
		}
	}
	delete(receipt, "exclude")
	receipt["settings_path"], err = json.Marshal(shared)
	if err == nil {
		receipt["artifacts"], err = json.Marshal(artifacts)
	}
	if err == nil {
		body, err = json.MarshalIndent(receipt, "", "  ")
	}
	if err != nil {
		return fmt.Errorf(encodeFailed, path, err)
	}
	return writeFile(path, body, fileMode)
}

// projectUnexclude takes the line of the installation out of the exclude
// file of the repository, as an earlier release never wrote it.
func projectUnexclude(repo string) error {
	path := projectExcludeFile(repo)
	body, err := readFile(path)
	if err != nil {
		return err
	}
	kept := bytes.Replace(body, []byte(projectExclude+"\n"), nil, 1)
	if bytes.Equal(kept, body) {
		return fmt.Errorf("the install wrote no exclude line: %w", ErrAcceptance)
	}
	return writeFile(path, kept, fileMode)
}

// exists reports whether anything is at path.
func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
