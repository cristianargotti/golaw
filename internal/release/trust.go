package release

import (
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"embed"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

// The files of a trust root directory: the certificate authorities and
// the public keys of the transparency logs, each a list of PEM blocks.
const (
	RootsFile = "roots.crt"
	RekorFile = "rekor.pub"
)

// The PEM block types each file holds.
const (
	blockCertificate = "CERTIFICATE"
	blockPublicKey   = "PUBLIC KEY"
)

// trustRoot is the public good trust root the binary carries: the root
// and the intermediate of its certificate authority, and the key of its
// transparency log, taken from the Sigstore trusted root (D11). The owner
// confirms these bytes against the Sigstore TUF root before every tag.
//
//go:embed trustroot/fulcio.crt trustroot/rekor.pub
var trustRoot embed.FS

// Sigstore is the embedded trust root of the public good instance, the
// one a release of the kit is signed under.
func Sigstore() (TrustRoot, error) {
	certs, err := trustRoot.ReadFile("trustroot/fulcio.crt")
	if err != nil {
		return TrustRoot{}, fmt.Errorf("%w: the embedded authorities: %w", ErrTrustRoot, err)
	}
	keys, err := trustRoot.ReadFile("trustroot/rekor.pub")
	if err != nil {
		return TrustRoot{}, fmt.Errorf("%w: the embedded log key: %w", ErrTrustRoot, err)
	}
	return parseTrustRoot(certs, keys)
}

// LoadTrustRoot reads roots.crt and rekor.pub of a directory, the trust
// root of a private Sigstore instance or of a test.
func LoadTrustRoot(dir string) (TrustRoot, error) {
	certs, err := os.ReadFile(filepath.Join(dir, RootsFile)) //nolint:gosec // the person names the trust root directory
	if err != nil {
		return TrustRoot{}, fmt.Errorf("%w: %w", ErrTrustRoot, err)
	}
	keys, err := os.ReadFile(filepath.Join(dir, RekorFile)) //nolint:gosec // the person names the trust root directory
	if err != nil {
		return TrustRoot{}, fmt.Errorf("%w: %w", ErrTrustRoot, err)
	}
	return parseTrustRoot(certs, keys)
}

// parseTrustRoot reads the two files of a trust root.
func parseTrustRoot(certs, keys []byte) (TrustRoot, error) {
	roots, err := authorities(certs)
	if err != nil {
		return TrustRoot{}, err
	}
	logs, err := logKeys(keys)
	if err != nil {
		return TrustRoot{}, err
	}
	return TrustRoot{Roots: roots, Rekor: logs}, nil
}

// authorities pools every certificate as an anchor, so an intermediate
// listed beside its root anchors a chain as well.
func authorities(data []byte) (*x509.CertPool, error) {
	ders, err := pemBlocks(RootsFile, blockCertificate, data)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	for _, der := range ders {
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrTrustRoot, RootsFile, err)
		}
		pool.AddCert(cert)
	}
	return pool, nil
}

// logKeys keys every log by its id, the hex SHA-256 of its DER form, the
// id a log entry names.
func logKeys(data []byte) (map[string]crypto.PublicKey, error) {
	ders, err := pemBlocks(RekorFile, blockPublicKey, data)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]crypto.PublicKey, len(ders))
	for _, der := range ders {
		key, err := x509.ParsePKIXPublicKey(der)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrTrustRoot, RekorFile, err)
		}
		id := sha256.Sum256(der)
		keys[hex.EncodeToString(id[:])] = key
	}
	return keys, nil
}

// pemBlocks are the bytes of every PEM block of a file, each of the one
// type the file holds; a file with none is refused.
func pemBlocks(name, kind string, data []byte) ([][]byte, error) {
	var out [][]byte
	for block, rest := pem.Decode(data); block != nil; block, rest = pem.Decode(rest) {
		if block.Type != kind {
			return nil, fmt.Errorf("%w: %s holds a %s block, not %s", ErrTrustRoot, name, block.Type, kind)
		}
		out = append(out, block.Bytes)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: %s holds no %s block", ErrTrustRoot, name, kind)
	}
	return out, nil
}
