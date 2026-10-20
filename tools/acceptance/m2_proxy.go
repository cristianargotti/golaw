package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/buildinfo"
)

const (
	m2ModulePath  = "github.com/cristianargotti/golaw"
	m2ProxyMoment = "2026-10-19T00:00:00Z"
)

func writeM2Proxy(root, proxy, version string, paths []string) (string, error) {
	if !buildinfo.IsRelease(version) {
		return "", fmt.Errorf("invalid M2 candidate version %q: %w", version, ErrAcceptance)
	}
	files, err := m2ArchiveFiles(root, paths)
	if err != nil {
		return "", err
	}
	var archive bytes.Buffer
	if err := writeM2Archive(&archive, version, files); err != nil {
		return "", err
	}
	if err := m2ProxyFiles(proxy, version, files, archive.Bytes()); err != nil {
		return "", err
	}
	return m2FileURL(proxy)
}

func m2ProxyFiles(proxy, version string, files []m2ArchiveFile, archive []byte) error {
	info, err := json.Marshal(struct {
		Version string `json:"Version"`
		Time    string `json:"Time"`
	}{"v" + version, m2ProxyMoment})
	if err != nil {
		return fmt.Errorf("encode M2 proxy info: %w", err)
	}
	var module []byte
	for _, file := range files {
		if file.name == moduleFile {
			module = file.data
		}
	}
	entries := []m2ArchiveFile{
		{name: "list", data: []byte("v" + version + "\n")},
		{name: "v" + version + ".mod", data: module},
		{name: "v" + version + ".info", data: info},
		{name: "v" + version + ".zip", data: archive},
	}
	dir := filepath.Join(proxy, filepath.FromSlash(m2ModulePath), "@v")
	for _, file := range entries {
		if err := writeFile(filepath.Join(dir, file.name), file.data, fileMode); err != nil {
			return err
		}
	}
	return nil
}

func m2FileURL(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve M2 proxy directory: %w", err)
	}
	slash := filepath.ToSlash(absolute)
	if !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	return (&url.URL{Scheme: "file", Path: slash}).String(), nil
}
