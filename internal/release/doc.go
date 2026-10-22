// Package release verifies a downloaded release of the kit inside the
// binary (specification section 4.2, contract D11): the asset line of the
// signed checksums.txt against the running executable, then the Sigstore
// bundle of checksums.txt under the pinned identity and the trust root,
// then the provenance attestation of the asset. A source build is never
// verified; it reports metadata, as the M1 row fixed. Nothing here reaches
// the network: every input is bytes the caller read from a directory.
package release
