package toolchain

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// execMode keeps an installed tool to its owner, like the rest of the tool
// directory, while the hook and lint can still start it.
const execMode = 0o700

// installBinary downloads, verifies and places one binary tool, unless the
// tool directory already holds it at the pin with its checksum.
func installBinary(ctx context.Context, o InstallOptions, t Tool) (Receipt, error) {
	resolver := Resolver{Dir: o.Dir, GOOS: o.GOOS, GOARCH: o.GOARCH, Env: o.Env, Run: o.Run}
	if rec, ok := resolver.verified(t); ok {
		return rec, say(o.Out, "%s %s já estava instalado e verificado em %s\n", t.Name, t.Version, rec.Path)
	}
	asset, err := assetFor(t, Platform(o.GOOS, o.GOARCH))
	if err != nil {
		return Receipt{}, err
	}
	data, err := download(ctx, o.Fetch, t, asset)
	if err != nil {
		return Receipt{}, err
	}
	if data, err = extract(data, asset); err != nil {
		return Receipt{}, err
	}
	path := BinPath(o.Dir, t, o.GOOS, o.GOARCH)
	if err := writeExecutable(path, data); err != nil {
		return Receipt{}, fmt.Errorf("%w: %s %s: %w", ErrInstall, t.Name, t.Version, err)
	}
	return record(o, t, path)
}

// assetFor answers the asset of platform; a refusal names the platforms
// upstream does publish, so the person sees why this machine has none.
func assetFor(t Tool, platform string) (Asset, error) {
	published := make([]string, 0, len(t.Assets))
	for _, asset := range t.Assets {
		if asset.Platform == platform {
			return asset, nil
		}
		published = append(published, asset.Platform)
	}
	return Asset{}, fmt.Errorf("%w: %s %s has no asset for %s (published for %s): the pipeline judges its files",
		ErrNoAsset, t.Name, t.Version, platform, strings.Join(published, ", "))
}

// download reads the whole asset into memory and refuses it unless its
// sha256 equals the pin, so a tampered byte never reaches the disk.
func download(ctx context.Context, fetch Fetcher, t Tool, asset Asset) ([]byte, error) {
	if fetch == nil {
		return nil, fmt.Errorf("%w: %s %s: no fetcher", ErrFetch, t.Name, t.Version)
	}
	body, err := fetch(ctx, asset.URL)
	if err != nil {
		if !errors.Is(err, ErrFetch) {
			err = fmt.Errorf("%w: %w", ErrFetch, err)
		}
		return nil, fmt.Errorf("%s %s: %w", t.Name, t.Version, err)
	}
	data, err := io.ReadAll(io.LimitReader(body, MaxAssetBytes+1))
	if err = errors.Join(err, body.Close()); err != nil {
		return nil, fmt.Errorf("%w: %s %s %s: %w", ErrFetch, t.Name, t.Version, asset.URL, err)
	}
	if len(data) > MaxAssetBytes {
		return nil, fmt.Errorf("%w: %s %s %s is larger than %d bytes", ErrFetch, t.Name, t.Version, asset.URL, MaxAssetBytes)
	}
	digest := sha256.Sum256(data)
	if observed := hex.EncodeToString(digest[:]); observed != asset.SHA256 {
		return nil, fmt.Errorf("%w: %s %s %s: expected %s, observed %s", ErrChecksum, t.Name, t.Version, asset.URL, asset.SHA256, observed)
	}
	return data, nil
}

// extract answers the executable an asset carries: the asset itself, or
// the pinned member of its archive.
func extract(data []byte, asset Asset) ([]byte, error) {
	var member []byte
	var err error
	switch asset.Archive {
	case "":
		return data, nil
	case archiveTarGz:
		member, err = tarMember(data, asset.Member)
	case archiveZip:
		member, err = zipMember(data, asset.Member)
	default:
		return nil, fmt.Errorf("%w: %s is a %s archive the installer cannot open", ErrArchive, asset.URL, asset.Archive)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrArchive, asset.URL, err)
	}
	if member == nil {
		return nil, fmt.Errorf("%w: %s has no member %s", ErrArchive, asset.URL, asset.Member)
	}
	return member, nil
}

// tarMember answers the regular entry named member, nil when none is.
func tarMember(data []byte, member string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	found, err := scanTar(tar.NewReader(gz), member)
	return found, errors.Join(err, gz.Close())
}

// scanTar reads entries until the member or the end of the archive.
func scanTar(archive *tar.Reader, member string) ([]byte, error) {
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}
		if header.Typeflag == tar.TypeReg && header.Name == member {
			return readMember(archive)
		}
	}
}

// zipMember answers the regular file named member, nil when none is.
func zipMember(data []byte, member string) ([]byte, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("zip: %w", err)
	}
	for _, file := range archive.File {
		if file.Name != member || !file.Mode().IsRegular() {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("zip %s: %w", member, err)
		}
		data, err := readMember(reader)
		return data, errors.Join(err, reader.Close())
	}
	return nil, nil
}

// readMember stops at MaxAssetBytes, so a decompression bomb inside a
// verified archive still cannot fill the memory.
func readMember(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxAssetBytes))
	if err != nil {
		return nil, fmt.Errorf("read member: %w", err)
	}
	return data, nil
}

func writeExecutable(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return fmt.Errorf("tool directory: %w", err)
	}
	return writeAtomic(path, data, execMode)
}
