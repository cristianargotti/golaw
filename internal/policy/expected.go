package policy

import (
	"context"
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// profilePrefix is how golaw.yml and the enrollment record write a
// profile digest.
const profilePrefix = "sha256:"

// Expected is an enrollment provisioned outside the checkout, as CI does
// with scan --enrolled (A3): it stands for the enrollment record.
type Expected struct {
	PolicyDigest  string
	ProfileDigest string // empty for core only
}

// ParseExpected reads the value of scan --enrolled: the policy digest,
// optionally followed by a comma and the profile digest, each 64 lower case
// hexadecimal characters; anything else is model.ErrUsage. The profile
// digest may carry the sha256: prefix golaw.yml writes, and is kept with
// it, the form the record and golaw.yml compare.
func ParseExpected(value string) (Expected, error) {
	policyDigest, profileDigest, withProfile := strings.Cut(value, ",")
	profileDigest = strings.TrimPrefix(profileDigest, profilePrefix)
	if !hexPattern.MatchString(policyDigest) || withProfile && !hexPattern.MatchString(profileDigest) {
		return Expected{}, fmt.Errorf("%w: the expected enrollment %q is not the policy digest, then a comma and the profile digest when a profile is enrolled, each 64 lower case hexadecimal characters as golaw config explain prints them",
			model.ErrUsage, value)
	}
	e := Expected{PolicyDigest: policyDigest}
	if withProfile {
		e.ProfileDigest = profilePrefix + profileDigest
	}
	return e, nil
}

// locate is Locate, or, with an expected enrollment, the repository the
// discoverer finds, enrolled with the expected digests: the state directory
// is not read, and a discovery that fails is ErrEnrollment, as it is for an
// enrolled repository (A14).
func (in Input) locate(ctx context.Context) (Location, error) {
	if in.Expected == nil {
		return Locate(ctx, in.Dirs.State, in.Dir, in.Discover)
	}
	found, err := in.Discover(ctx, in.Dir)
	if err != nil {
		return Location{}, fmt.Errorf("%w: %s, enrolled as the pipeline expects: discovery failed: %w", model.ErrEnrollment, in.Dir, err)
	}
	record := Record{Root: found.Root, PolicyDigest: in.Expected.PolicyDigest, ProfileDigest: in.Expected.ProfileDigest}
	return Location{Root: found.Root, Head: found.Head, Record: &record}, nil
}

// unexpected refuses a policy in force whose digests are not the ones the
// pipeline provisions, naming both, so the person reviews the change and,
// once it is approved, provisions the digests golaw config explain prints.
func (l *Loaded) unexpected(profile string) error {
	return fmt.Errorf("%w: %s of %s has the policy digest %s and the profile digest %q, and the pipeline expects %s and %q: review the change, then provision the digests golaw config explain prints",
		model.ErrEnrollment, FileName, l.Root, l.Policy.Digest, profile, l.Record.PolicyDigest, l.Record.ProfileDigest)
}
