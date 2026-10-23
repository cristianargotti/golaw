package release

import (
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"slices"
	"strings"
	"time"
)

// signer is the certificate of a bundle once it verified for the release:
// its log entry verifies, it chains to the trust root with the code
// signing usage at the time the log integrated the signature, and it names
// the pinned identity at the release tag (D11).
func signer(b sigstoreBundle, in Inputs) (*x509.Certificate, error) {
	at, err := logged(b.VerificationMaterial.TlogEntries, in.Trust)
	if err != nil {
		return nil, err
	}
	cert, err := chained(b.VerificationMaterial.Certificate, in.Trust, at)
	if err != nil {
		return nil, err
	}
	if err := named(cert, in.Identity, in.Version); err != nil {
		return nil, err
	}
	return cert, nil
}

// chained parses the leaf of the bundle and verifies its chain at a time:
// short lived leaves are judged when the log saw them, never at the clock.
func chained(raw *rawCertificate, trust TrustRoot, at time.Time) (*x509.Certificate, error) {
	if raw == nil {
		return nil, fmt.Errorf("%w: the bundle carries no certificate", errSignature)
	}
	der, err := decode64("the certificate", raw.RawBytes)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("%w: the certificate does not parse: %w", errSignature, err)
	}
	opts := x509.VerifyOptions{Roots: trust.Roots, CurrentTime: at, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}}
	if _, err := cert.Verify(opts); err != nil {
		return nil, fmt.Errorf("%w: the certificate does not chain to the trust root at %s: %w", errSignature, at.Format(time.RFC3339), err)
	}
	return cert, nil
}

// named requires the subject alternative name of the release tag and the
// pinned OIDC issuer.
func named(cert *x509.Certificate, id Identity, version string) error {
	want := id.Subject(version)
	subjects := make([]string, 0, len(cert.URIs))
	for _, uri := range cert.URIs {
		subjects = append(subjects, uri.String())
	}
	if !slices.Contains(subjects, want) {
		return fmt.Errorf("%w: the certificate names %q and this release must be signed by %s", errIdentity, strings.Join(subjects, ", "), want)
	}
	if issuer := issuerOf(cert); issuer != id.Issuer {
		return fmt.Errorf("%w: the certificate was issued for %q and this release must come from %s", errIdentity, issuer, id.Issuer)
	}
	return nil
}

// issuerOf reads the OIDC issuer the certificate authority wrote: the
// DER UTF8String of 1.3.6.1.4.1.57264.1.8, else the raw text of the first
// version 1.3.6.1.4.1.57264.1.1; empty when the leaf carries neither.
func issuerOf(cert *x509.Certificate) string {
	var raw string
	for _, ext := range cert.Extensions {
		switch {
		case ext.Id.Equal(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 8}):
			var text string
			if _, err := asn1.UnmarshalWithParams(ext.Value, &text, "utf8"); err == nil {
				return text
			}
		case ext.Id.Equal(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}):
			raw = string(ext.Value)
		}
	}
	return raw
}
