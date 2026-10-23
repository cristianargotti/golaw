package release_test

import (
	"crypto"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/release"
	"github.com/cristianargotti/golaw/internal/releasetest"
)

// entryEdit changes one field of the log entry of the bundle.
func entryEdit(key string, value any) func(*testing.T, *release.Inputs) {
	return func(t *testing.T, in *release.Inputs) {
		t.Helper()
		in.Bundle = edited(t, in.Bundle, func(doc map[string]any) {
			entry := object(t, doc, "verificationMaterial", "tlogEntries", "0")
			if value == nil {
				delete(entry, key)
				return
			}
			entry[key] = value
		})
	}
}

func TestVerifyRefusesALogEntryTheLogDidNotSign(t *testing.T) {
	t.Parallel()
	const unsigned = "the ECDSA signature does not verify for the signed entry timestamp of the log"
	cases := map[string]struct {
		change func(*testing.T, *release.Inputs)
		detail string
	}{
		"another time":           {entryEdit("integratedTime", "1790859600"), unsigned},
		"another index":          {entryEdit("logIndex", "7"), unsigned},
		"another body":           {entryEdit("canonicalizedBody", "e30="), unsigned},
		"another timestamp":      {entryEdit("inclusionPromise", map[string]any{"signedEntryTimestamp": "MEUCIQ=="}), unsigned},
		"no timestamp":           {entryEdit("inclusionPromise", nil), "carries no signed entry timestamp"},
		"a timestamp not base64": {entryEdit("inclusionPromise", map[string]any{"signedEntryTimestamp": "%%%"}), "the signed entry timestamp is not base64"},
		"a body not base64":      {entryEdit("canonicalizedBody", "%%%"), "the log entry body is not base64"},
		"a log id not base64":    {entryEdit("logId", map[string]any{"keyId": "%%%"}), "the log id is not base64"},
		"a log it does not hold": {func(t *testing.T, in *release.Inputs) {
			t.Helper()
			in.Trust.Rekor = releasetest.Generate(t, []byte(binary), "1.2.3").Inputs(t).Trust.Rekor
		}, "which the trust root does not hold"},
		"a log key of another type": {func(t *testing.T, in *release.Inputs) {
			t.Helper()
			for id := range in.Trust.Rekor {
				in.Trust.Rekor = map[string]crypto.PublicKey{id: "not a key"}
			}
		}, "the key is string and this version verifies ECDSA keys only"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in, _ := material(t)
			tc.change(t, &in)
			v := release.Verify(in)
			if v.State != release.StateRed || v.Code != release.CodeSignatureInvalid || !strings.Contains(v.Detail, tc.detail) {
				t.Fatalf("Verify = %+v, want signature-invalid with %q", v, tc.detail)
			}
		})
	}
}

func TestVerifyJudgesTheLeafWhenTheLogSawIt(t *testing.T) {
	t.Parallel()
	in, _ := material(t)
	if v := release.Verify(in); v.Code != release.CodeUnverified || v.Detail != "the attestation is not verified yet" {
		t.Fatalf("a logged leaf ten minutes long verifies whatever the clock says: %+v", v)
	}
}

// The log signed an entry of another signature by the same leaf: the
// entry is real, but it does not record the signature of this bundle, so
// it binds neither the signature nor its time.
func TestVerifyRefusesALogEntryOfAnotherSignature(t *testing.T) {
	t.Parallel()
	in, m := material(t)
	in.Bundle = m.Tamper(t, "log-body").Inputs(t).Bundle
	if v := release.Verify(in); v.Code != release.CodeSignatureInvalid || !strings.Contains(v.Detail, "the log entry does not record this signature of checksums.txt") {
		t.Fatalf("Verify = %+v", v)
	}
}
