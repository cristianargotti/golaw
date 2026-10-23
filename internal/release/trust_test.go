package release_test

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/release"
	"github.com/cristianargotti/golaw/internal/releasetest"
)

// publicGoodLog is the id of the public good transparency log, the hex
// SHA-256 of its key, as the Sigstore trusted root names it.
const publicGoodLog = "c0d23d6ad406973f9559f3ba2d1ca01f84147d8ffc5b8445c224f98b9591801d"

func TestSigstoreIsThePublicGoodTrustRoot(t *testing.T) {
	t.Parallel()
	trust, err := release.Sigstore()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := trust.Rekor[publicGoodLog]; !ok || len(trust.Rekor) != 1 {
		t.Fatalf("the log keys are %v", trust.Rekor)
	}
	data, err := os.ReadFile(filepath.Join("trustroot", "fulcio.crt"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for block, rest := pem.Decode(data); block != nil; block, rest = pem.Decode(rest) {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatal(err)
		}
		at := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
		opts := x509.VerifyOptions{Roots: trust.Roots, CurrentTime: at, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}}
		if _, err := cert.Verify(opts); err != nil {
			t.Errorf("%s anchors the pool: %v", cert.Subject, err)
		}
		names = append(names, cert.Subject.CommonName)
	}
	if strings.Join(names, ",") != "sigstore-intermediate,sigstore" {
		t.Fatalf("the authorities are %v", names)
	}
}

func TestTheEmbeddedRootRefusesATestSigner(t *testing.T) {
	t.Parallel()
	in, _ := material(t)
	trust, err := release.Sigstore()
	if err != nil {
		t.Fatal(err)
	}
	in.Trust = trust
	if v := release.Verify(in); v.Code != release.CodeSignatureInvalid || !strings.Contains(v.Detail, "trust root does not hold") {
		t.Fatalf("Verify = %+v", v)
	}
}

func TestLoadTrustRootReadsADirectory(t *testing.T) {
	t.Parallel()
	m := releasetest.Generate(t, []byte(binary), "1.2.3")
	trust, err := release.LoadTrustRoot(m.Trust)
	if err != nil || trust.Roots == nil || len(trust.Rekor) != 1 {
		t.Fatalf("LoadTrustRoot = %+v, %v", trust, err)
	}
	in := m.Inputs(t)
	in.Trust = trust
	if v := release.Verify(in); v.Code != release.CodeVerified {
		t.Fatalf("Verify = %+v", v)
	}
}

func TestLoadTrustRootRefusesWhatIsNotATrustRoot(t *testing.T) {
	t.Parallel()
	m := releasetest.Generate(t, []byte(binary), "1.2.3")
	certs, err := os.ReadFile(filepath.Join(m.Trust, release.RootsFile))
	if err != nil {
		t.Fatal(err)
	}
	keys, err := os.ReadFile(filepath.Join(m.Trust, release.RekorFile))
	if err != nil {
		t.Fatal(err)
	}
	crl := "-----BEGIN X509 CRL-----\nAAAA\n-----END X509 CRL-----\n"
	broken := "-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n"
	cases := map[string]struct {
		roots, logs string
		want        string
	}{
		"no authority":                 {"", string(keys), "roots.crt holds no CERTIFICATE block"},
		"no log key":                   {string(certs), "", "rekor.pub holds no PUBLIC KEY block"},
		"another block":                {string(certs) + crl, string(keys), "roots.crt holds a X509 CRL block, not CERTIFICATE"},
		"a broken certificate":         {broken, string(keys), "roots.crt: x509"},
		"a broken key":                 {string(certs), strings.ReplaceAll(broken, "CERTIFICATE", "PUBLIC KEY"), "invalid trust root: rekor.pub: "},
		"keys in place of authorities": {string(keys), string(keys), "roots.crt holds a PUBLIC KEY block"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for file, content := range map[string]string{release.RootsFile: tc.roots, release.RekorFile: tc.logs} {
				if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := release.LoadTrustRoot(dir)
			if !errors.Is(err, release.ErrTrustRoot) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadTrustRoot = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoadTrustRootNamesAMissingFile(t *testing.T) {
	t.Parallel()
	m := releasetest.Generate(t, []byte(binary), "1.2.3")
	onlyRoots := t.TempDir()
	certs, err := os.ReadFile(filepath.Join(m.Trust, release.RootsFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(onlyRoots, release.RootsFile), certs, 0o600); err != nil {
		t.Fatal(err)
	}
	for name, dir := range map[string]string{
		"absent": filepath.Join(t.TempDir(), "absent"), "a file": filepath.Join(m.Trust, release.RootsFile), "no log key file": onlyRoots,
	} {
		if _, err := release.LoadTrustRoot(dir); !errors.Is(err, release.ErrTrustRoot) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
