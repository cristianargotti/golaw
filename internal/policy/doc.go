// Package policy turns golaw.yml into the one immutable effective policy
// of a run: strict decoding, validation against the core floors,
// compilation, the digest, the tightening check between the policy in
// force and the proposed next one, the enrollment record outside the
// checkout, the profile stamp and the explanation of every value. The
// policy in force is read from HEAD, never from the working tree, or from
// the bootstrap record on an unborn branch (A2).
package policy
