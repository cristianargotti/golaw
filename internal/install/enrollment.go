package install

import (
	"context"

	"github.com/cristianargotti/golaw/internal/model"
)

// EnrollmentWriter keeps enrollment persistence with the policy owner.
//
//nolint:iface // Enrollment implementations belong to the policy package.
type EnrollmentWriter interface {
	WriteEnrollment(
		ctx context.Context,
		identity, policyDigest, profileDigest string,
	) (model.Enrollment, error)
}
