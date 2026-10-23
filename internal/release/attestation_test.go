package release_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/release"
	"github.com/cristianargotti/golaw/internal/releasetest"
)

func TestVerifyAnswersVerifiedWithTheSigner(t *testing.T) {
	t.Parallel()
	in, m := material(t)
	want := release.Verdict{
		State: release.StateOK, Code: release.CodeVerified, Digest: in.BinaryDigest,
		Detail: "checksums.txt, its bundle and the attestation of " + m.Asset + " verify",
		Signer: "https://example.test/.github/workflows/release.yml@refs/tags/v1.2.3",
	}
	if v := release.Verify(in); v != want {
		t.Fatalf("Verify = %+v, want %+v", v, want)
	}
}

func TestVerifyReadsEveryBundleOfADownloadedFile(t *testing.T) {
	t.Parallel()
	in, m := material(t)
	other := m.Tamper(t, "attestation").Inputs(t).Attestation
	for name, file := range map[string][]byte{
		"one per line, the good one second": bytes.Join([][]byte{other, in.Attestation}, []byte("\n")),
		"one document over several lines":   bytes.ReplaceAll(in.Attestation, []byte(`","`), []byte("\",\n\"")),
	} {
		got := in
		got.Attestation = file
		if v := release.Verify(got); v.Code != release.CodeVerified {
			t.Errorf("%s: %+v", name, v)
		}
	}
	got := in
	got.Attestation = bytes.Join([][]byte{other, m.Tamper(t, "identity").Inputs(t).Attestation}, []byte("\n"))
	if v := release.Verify(got); v.Code != release.CodeAttestationMismatch {
		t.Errorf("no good bundle answers the failure of the first: %+v", v)
	}
}

// attestationCase changes the attestation of a good release.
type attestationCase struct {
	change func(*testing.T, *release.Inputs, releasetest.Material)
	code   string
	detail string
}

func envelopeEdit(key string, value any) func(*testing.T, *release.Inputs, releasetest.Material) {
	return func(t *testing.T, in *release.Inputs, _ releasetest.Material) {
		t.Helper()
		in.Attestation = edited(t, in.Attestation, func(doc map[string]any) {
			object(t, doc, "dsseEnvelope")[key] = value
		})
	}
}

func attestationCases() map[string]attestationCase {
	tampered := func(what string) func(*testing.T, *release.Inputs, releasetest.Material) {
		return func(t *testing.T, in *release.Inputs, m releasetest.Material) {
			t.Helper()
			in.Attestation = m.Tamper(t, what).Inputs(t).Attestation
		}
	}
	mismatch, invalid := release.CodeAttestationMismatch, "invalid signature: "
	return map[string]attestationCase{
		"missing":        {tampered("no-attestation"), release.CodeAttestationMissing, "download it with gh attestation download"},
		"another logged": {tampered("attestation-log-body"), mismatch, "the log entry does not record this envelope"},
		"another digest": {tampered("attestation"), mismatch, "no subject of the statement is"},
		"another lane":   {tampered("identity"), release.CodeIdentityMismatch, "workflows/other.yml@refs/tags/v1.2.3"},
		"the bundle of checksums": {func(_ *testing.T, in *release.Inputs, _ releasetest.Material) {
			in.Attestation = in.Bundle
		}, mismatch, "no signed in-toto envelope"},
		"another payload type":   {envelopeEdit("payloadType", "text/plain"), mismatch, "no signed in-toto envelope"},
		"another payload":        {envelopeEdit("payload", "e30="), mismatch, "the ECDSA signature does not verify over the envelope"},
		"a payload not base64":   {envelopeEdit("payload", "%%%"), mismatch, "the payload is not base64"},
		"no signature":           {envelopeEdit("signatures", []any{}), mismatch, "no signed in-toto envelope"},
		"a signature not base64": {envelopeEdit("signatures", []any{map[string]any{"sig": "%%%"}}), mismatch, "the envelope signature is not base64"},
		"blank":                  {func(_ *testing.T, in *release.Inputs, _ releasetest.Material) { in.Attestation = []byte("\n") }, mismatch, invalid + "the file holds no bundle"},
		"not JSON":               {func(_ *testing.T, in *release.Inputs, _ releasetest.Material) { in.Attestation = []byte("{") }, mismatch, invalid + "the bundle does not decode"},
	}
}

func TestVerifyRefusesAnAttestationThatDoesNotNameTheBinary(t *testing.T) {
	t.Parallel()
	for name, tc := range attestationCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in, m := material(t)
			tc.change(t, &in, m)
			v := release.Verify(in)
			if v.State != release.StateRed || v.Code != tc.code || !strings.Contains(v.Detail, tc.detail) || v.Signer != "" {
				t.Fatalf("Verify = %+v, want %s with %q", v, tc.code, tc.detail)
			}
		})
	}
}
