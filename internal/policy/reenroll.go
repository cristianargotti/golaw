package policy

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// CheckReenrollment refuses to move a record to a policy that weakens the
// one it enrolled (A2): the record holds the bytes of its policy, and a
// new policy replaces them only as a tightening, so a reviewed change can
// be enrolled while a weaker file pulled into the checkout stays refused.
// A record that holds no policy gives nothing to compare, like no record
// at all, so the new policy is enrolled afresh; bytes that do not decode,
// or do not hash to the record's digest, are a damaged record.
func CheckReenrollment(r Record, next File, schemas Schemas) error {
	if r.Bootstrap == "" {
		return nil
	}
	enrolled, err := Decode([]byte(r.Bootstrap))
	if err != nil || Digest(enrolled) != r.PolicyDigest {
		return fmt.Errorf("%w: the policy the record of %s enrolled is damaged", model.ErrEnrollment, r.Root)
	}
	if err := CheckTightening(enrolled, next, schemas); err != nil {
		return fmt.Errorf("%w: %s of %s weakens the policy the record enrolled: %w", model.ErrEnrollment, FileName, r.Root, err)
	}
	return nil
}
