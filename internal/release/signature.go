package release

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/sha256"
	"fmt"
)

// checksumsSigned proves the bundle of checksums.txt: its certificate is
// the signer of the release and its signature verifies over the SHA-256
// of the very file the person downloaded.
func checksumsSigned(in Inputs) error {
	b, err := decodeBundle(in.Bundle)
	if err != nil {
		return err
	}
	if b.MessageSignature == nil {
		return fmt.Errorf("%w: the bundle carries no message signature", errSignature)
	}
	cert, err := signer(b, in)
	if err != nil {
		return err
	}
	sig, err := decode64("the signature", b.MessageSignature.Signature)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(in.Checksums)
	if err := verifyECDSA(cert.PublicKey, sum[:], sig); err != nil {
		return fmt.Errorf("%w over %s", err, ChecksumsFile)
	}
	return nil
}

// verifyECDSA verifies an ASN.1 ECDSA signature over a digest, the one
// key type the public signing service and its log use.
func verifyECDSA(key crypto.PublicKey, digest, sig []byte) error {
	pub, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("%w: the key is %T and this version verifies ECDSA keys only", errSignature, key)
	}
	if !ecdsa.VerifyASN1(pub, digest, sig) {
		return fmt.Errorf("%w: the ECDSA signature does not verify", errSignature)
	}
	return nil
}
