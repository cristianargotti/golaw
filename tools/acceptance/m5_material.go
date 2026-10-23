package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/release"
)

// The files of generated material: the release directory doctor --release
// reads and the trust root directory --trust-root names.
const (
	m5ReleaseDir = "release"
	m5TrustDir   = "trust"
	m5RootsFile  = "roots.crt"
	m5RekorFile  = "rekor.pub"
	m5ZeroDigest = "0000000000000000000000000000000000000000000000000000000000000000"
)

// m5Material is the release a person downloads for one asset, generated
// under a test authority: a copy of the construction of releasetest,
// because the tool never imports a test package.
type m5Material struct {
	dir, trust, asset string
}

// m5Moment is the integrated time of every generated entry: the tool reads
// no clock, and a certificate is verified at the time of its entry (D11).
func m5Moment() time.Time { return time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC) }

// m5Generate writes checksums.txt, its Sigstore bundle and the attestation
// of the asset whose bytes are binary, signed for identity at the
// candidate's version, and the trust root that verifies them.
func m5Generate(dir string, binary []byte, identity release.Identity) (m5Material, error) {
	m := m5Material{
		dir: filepath.Join(dir, m5ReleaseDir), trust: filepath.Join(dir, m5TrustDir),
		asset: release.AssetName(m5CandidateVersion, runtime.GOOS, runtime.GOARCH),
	}
	authority, err := m5NewAuthority(m5Moment())
	if err != nil {
		return m5Material{}, err
	}
	leaf, key, err := authority.leaf(identity, m5CandidateVersion)
	if err != nil {
		return m5Material{}, err
	}
	sum := sha256.Sum256(binary)
	digest := hex.EncodeToString(sum[:])
	checksums := []byte(digest + "  " + m.asset + "\n")
	bundle, err := authority.messageBundle(checksums, leaf, key)
	if err != nil {
		return m5Material{}, err
	}
	attestation, err := authority.attestationBundle(m.asset, digest, leaf, key)
	if err != nil {
		return m5Material{}, err
	}
	files := map[string][]byte{
		release.ChecksumsFile: checksums, release.ChecksumsFile + release.BundleSuffix: bundle,
		m.asset + release.BundleSuffix: attestation,
	}
	return m, m5WriteMaterial(m, files, authority)
}

// m5WriteMaterial writes the release files and the trust root.
func m5WriteMaterial(m m5Material, files map[string][]byte, authority *m5Authority) error {
	for name, body := range files {
		if err := writeFile(filepath.Join(m.dir, name), body, fileMode); err != nil {
			return err
		}
	}
	roots, rekor, err := authority.trustFiles()
	if err != nil {
		return err
	}
	if stepErr := writeFile(filepath.Join(m.trust, m5RootsFile), roots, fileMode); stepErr != nil {
		return stepErr
	}
	return writeFile(filepath.Join(m.trust, m5RekorFile), rekor, fileMode)
}

// m5Tamper rewrites one piece of generated material, the way a download
// can arrive altered: "checksums" (the asset line names another digest),
// "signature" (the last byte of the bundle signature), "attestation" (the
// statement changed under its signature) or "no-attestation" (the file
// absent).
func m5Tamper(m m5Material, what string) error {
	switch what {
	case "checksums":
		return m5Rewrite(filepath.Join(m.dir, release.ChecksumsFile), func(body []byte) ([]byte, error) {
			_, name, _ := strings.Cut(string(body), "  ")
			return []byte(m5ZeroDigest + "  " + name), nil
		})
	case m5KeySignature:
		return m5Rewrite(filepath.Join(m.dir, release.ChecksumsFile+release.BundleSuffix), m5FlipSignature)
	case "attestation":
		return m5Rewrite(filepath.Join(m.dir, m.asset+release.BundleSuffix), m5ChangeStatement)
	case "no-attestation":
		if err := os.Remove(filepath.Join(m.dir, m.asset+release.BundleSuffix)); err != nil {
			return fmt.Errorf("remove the attestation: %w", err)
		}
		return nil
	}
	return fmt.Errorf("unknown tamper %q: %w", what, ErrAcceptance)
}

func m5Rewrite(path string, change func([]byte) ([]byte, error)) error {
	body, err := readFile(path)
	if err != nil {
		return err
	}
	changed, err := change(body)
	if err != nil {
		return err
	}
	return writeFile(path, changed, fileMode)
}

// m5Object is the JSON object under key, an error when it is absent or of
// another kind.
func m5Object(parent map[string]any, key string) (map[string]any, error) {
	object, ok := parent[key].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("the material has no object %s: %w", key, ErrAcceptance)
	}
	return object, nil
}

// m5Decoded is the base64 string under key, decoded.
func m5Decoded(parent map[string]any, key string) ([]byte, error) {
	encoded, ok := parent[key].(string)
	if !ok {
		return nil, fmt.Errorf("the material has no string %s: %w", key, ErrAcceptance)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf(decodeFormat, key, err)
	}
	return decoded, nil
}

// m5FlipSignature keeps the bundle well formed and its signature a valid
// encoding that no longer matches checksums.txt.
func m5FlipSignature(body []byte) ([]byte, error) {
	var bundle map[string]any
	if err := json.Unmarshal(body, &bundle); err != nil {
		return nil, fmt.Errorf("decode the bundle: %w", err)
	}
	message, err := m5Object(bundle, "messageSignature")
	if err != nil {
		return nil, err
	}
	signature, err := m5Decoded(message, m5KeySignature)
	if err != nil || len(signature) == 0 {
		return nil, fmt.Errorf("decode the bundle signature: %w", ErrAcceptance)
	}
	signature[len(signature)-1] ^= 0x01
	message[m5KeySignature] = base64.StdEncoding.EncodeToString(signature)
	return m5JSON(bundle)
}

// m5ChangeStatement names another digest in the attested statement and
// leaves its signature as it was.
func m5ChangeStatement(body []byte) ([]byte, error) {
	var bundle map[string]any
	if err := json.Unmarshal(body, &bundle); err != nil {
		return nil, fmt.Errorf("decode the attestation: %w", err)
	}
	envelope, err := m5Object(bundle, "dsseEnvelope")
	if err != nil {
		return nil, err
	}
	payload, err := m5Decoded(envelope, "payload")
	if err != nil {
		return nil, err
	}
	var statement map[string]any
	if decodeErr := json.Unmarshal(payload, &statement); decodeErr != nil {
		return nil, fmt.Errorf("decode the statement: %w", decodeErr)
	}
	statement["subject"] = []any{map[string]any{"name": "golaw", m5KeyDigest: map[string]any{m5SHA256: m5ZeroDigest}}}
	changed, err := json.Marshal(statement)
	if err != nil {
		return nil, fmt.Errorf("encode the statement: %w", err)
	}
	envelope["payload"] = base64.StdEncoding.EncodeToString(changed)
	return m5JSON(bundle)
}
