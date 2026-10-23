package release_test

import (
	"crypto"
	"crypto/x509"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/release"
	"github.com/cristianargotti/golaw/internal/releasetest"
)

// bundleCase changes one input of a good release and names the verdict.
type bundleCase struct {
	change func(t *testing.T, in *release.Inputs, m releasetest.Material)
	code   string
	detail string
}

// bundleCases are the ways the bundle of checksums.txt fails; each answers
// red with the bundle and the cause named.
func bundleCases() map[string]bundleCase {
	setEntry := func(key string, value any) func(*testing.T, *release.Inputs, releasetest.Material) {
		return func(t *testing.T, in *release.Inputs, _ releasetest.Material) {
			t.Helper()
			in.Bundle = edited(t, in.Bundle, func(doc map[string]any) {
				object(t, doc, "verificationMaterial", "tlogEntries", "0")[key] = value
			})
		}
	}
	return map[string]bundleCase{
		"signed other bytes": {func(t *testing.T, in *release.Inputs, m releasetest.Material) {
			t.Helper()
			in.Bundle = m.Tamper(t, "signature").Inputs(t).Bundle
		}, release.CodeSignatureInvalid, "the ECDSA signature does not verify over checksums.txt"},
		"another lane": {func(t *testing.T, in *release.Inputs, m releasetest.Material) {
			t.Helper()
			in.Bundle = m.Tamper(t, "identity").Inputs(t).Bundle
		}, release.CodeIdentityMismatch, "workflows/other.yml@refs/tags/v1.2.3"},
		"another tag": {func(_ *testing.T, in *release.Inputs, _ releasetest.Material) {
			in.Version = "1.2.4"
		}, release.CodeIdentityMismatch, "must be signed by https://example.test/.github/workflows/release.yml@refs/tags/v1.2.4"},
		"another issuer": {func(_ *testing.T, in *release.Inputs, _ releasetest.Material) {
			in.Identity.Issuer = release.DefaultIssuer
		}, release.CodeIdentityMismatch, `issued for "https://issuer.example.test"`},
		"another authority": {func(t *testing.T, in *release.Inputs, _ releasetest.Material) {
			t.Helper()
			in.Trust.Roots = releasetest.Generate(t, []byte(binary), "1.2.3").Inputs(t).Trust.Roots
		}, release.CodeSignatureInvalid, "does not chain to the trust root at 2026-10-01T12:00:00Z"},
		"after the leaf expired": setEntryCase(setEntry("integratedTime", "1790859600"), "does not chain to the trust root at 2026-10-01T13:00:00Z"),
		"no log entry": {func(t *testing.T, in *release.Inputs, _ releasetest.Material) {
			t.Helper()
			in.Bundle = edited(t, in.Bundle, func(doc map[string]any) {
				object(t, doc, "verificationMaterial")["tlogEntries"] = []any{}
			})
		}, release.CodeSignatureInvalid, "no transparency log entry"},
		"a time that is not a number": setEntryCase(setEntry("integratedTime", "soon"), "not an integer"),
		"no certificate": {func(t *testing.T, in *release.Inputs, _ releasetest.Material) {
			t.Helper()
			in.Bundle = edited(t, in.Bundle, func(doc map[string]any) {
				delete(object(t, doc, "verificationMaterial"), "certificate")
			})
		}, release.CodeSignatureInvalid, "the bundle carries no certificate"},
		"a certificate that is not base64":  certificateCase("%%%", "the certificate is not base64"),
		"a certificate that does not parse": certificateCase("AAAA", "the certificate does not parse"),
		"no message signature": {func(t *testing.T, in *release.Inputs, _ releasetest.Material) {
			t.Helper()
			in.Bundle = edited(t, in.Bundle, func(doc map[string]any) { delete(doc, "messageSignature") })
		}, release.CodeSignatureInvalid, "no message signature"},
		"a signature that is not base64": {func(t *testing.T, in *release.Inputs, _ releasetest.Material) {
			t.Helper()
			in.Bundle = edited(t, in.Bundle, func(doc map[string]any) {
				object(t, doc, "messageSignature")["signature"] = "%%%"
			})
		}, release.CodeSignatureInvalid, "the signature is not base64"},
		"not JSON": {func(_ *testing.T, in *release.Inputs, _ releasetest.Material) {
			in.Bundle = []byte("not json")
		}, release.CodeSignatureInvalid, "the bundle does not decode"},
	}
}

func setEntryCase(change func(*testing.T, *release.Inputs, releasetest.Material), detail string) bundleCase {
	return bundleCase{change, release.CodeSignatureInvalid, detail}
}

func certificateCase(raw, detail string) bundleCase {
	return bundleCase{func(t *testing.T, in *release.Inputs, _ releasetest.Material) {
		t.Helper()
		in.Bundle = edited(t, in.Bundle, func(doc map[string]any) {
			object(t, doc, "verificationMaterial", "certificate")["rawBytes"] = raw
		})
	}, release.CodeSignatureInvalid, detail}
}

func TestVerifyRefusesABundleThatDoesNotSignTheRelease(t *testing.T) {
	t.Parallel()
	for name, tc := range bundleCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in, m := material(t)
			tc.change(t, &in, m)
			v := release.Verify(in)
			if v.State != release.StateRed || v.Code != tc.code || !strings.HasPrefix(v.Detail, "checksums.txt.sigstore.json: ") || !strings.Contains(v.Detail, tc.detail) {
				t.Fatalf("Verify = %+v, want %s with %q", v, tc.code, tc.detail)
			}
		})
	}
}

func TestVerifyGoesPastAGoodBundle(t *testing.T) {
	t.Parallel()
	in, _ := material(t)
	if v := release.Verify(in); v.State != release.StateUnknown || v.Code != release.CodeUnverified || v.Signer != "" || v.Digest != in.BinaryDigest {
		t.Fatalf("Verify = %+v", v)
	}
}

func TestVerifyNeedsAnAuthorityAndALogKey(t *testing.T) {
	t.Parallel()
	in, _ := material(t)
	noLog, noRoots := in, in
	noLog.Trust.Rekor = map[string]crypto.PublicKey{}
	noRoots.Trust.Roots = nil
	for name, broken := range map[string]release.Inputs{"no log key": noLog, "no authority": noRoots} {
		if v := release.Verify(broken); v.Code != release.CodeTrustRootInvalid {
			t.Errorf("%s: %+v", name, v)
		}
	}
	empty := in
	empty.Trust.Roots = x509.NewCertPool()
	if v := release.Verify(empty); v.Code != release.CodeSignatureInvalid {
		t.Errorf("an authority pool without the signer's authority: %+v", v)
	}
}
