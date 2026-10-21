package model

import "errors"

// The sentinel errors every package of the kit matches with errors.Is. They
// live here so that cmd, hook and scan map them to exit codes and denials
// without importing the package that raised them.
var (
	// ErrUsage marks a caller mistake: an invalid flag, a malformed input, an
	// input without a snapshot or a policy. A command exits 2 on it.
	ErrUsage = errors.New("usage")
	// ErrInvalidSpec marks a check specification the registry refuses.
	ErrInvalidSpec = errors.New("invalid check spec")
	// ErrDuplicateCheck marks a second registration of one check id.
	ErrDuplicateCheck = errors.New("duplicate check id")
	// ErrUnknownProof marks a check whose proof key is absent from the catalog.
	ErrUnknownProof = errors.New("unknown proof name")
	// ErrNotInSnapshot marks a read of a path the snapshot does not hold.
	ErrNotInSnapshot = errors.New("path not in snapshot")
	// ErrUnreadable marks a path the snapshot holds but cannot read.
	ErrUnreadable = errors.New("path unreadable")
	// ErrTooLarge marks a read beyond MaxReadBytes; the check that meets it
	// answers Unmeasured, never clean.
	ErrTooLarge = errors.New("file larger than the read bound")
	// ErrPolicy marks a golaw.yml that is missing, malformed or weakened.
	ErrPolicy = errors.New("invalid policy")
	// ErrEnrollment marks an enrolled repository whose policy does not match
	// its enrollment record.
	ErrEnrollment = errors.New("enrollment mismatch")
	// ErrDeadline marks an answer that could not be reached before the
	// internal deadline; a hook turns it into a denial.
	ErrDeadline = errors.New("internal deadline passed")
	// ErrCannotJudge marks a hook input the kit cannot judge: malformed
	// stdin, an unreadable target, an unknown mutation schema.
	ErrCannotJudge = errors.New("cannot judge")
	// ErrOutsideSubset marks a configuration construct outside the subset a
	// policy check declares it reads: a stated limit, never a finding.
	ErrOutsideSubset = errors.New("outside the declared subset, a stated limit; docs/rules.md names the rewrite")
)
