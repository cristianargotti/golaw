package install

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/cristianargotti/golaw/internal/model"
)

// Apply makes the diff observable before enrollment or artifact writes.
func Apply(ctx context.Context, plan Plan, out io.Writer, enrollment EnrollmentWriter) error {
	if err := WriteDiff(out, plan.Changes); err != nil {
		return err
	}
	if err := verifyChanges(plan.Changes); err != nil {
		return err
	}
	if !plan.Uninstall {
		if err := enroll(ctx, plan, enrollment); err != nil {
			return err
		}
	}
	for _, change := range plan.Changes {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("apply installation: %w", err)
		}
		if err := applyChange(change); err != nil {
			return err
		}
	}
	return nil
}

func enroll(ctx context.Context, plan Plan, writer EnrollmentWriter) error {
	if writer == nil {
		return fmt.Errorf("enrollment writer: %w", ErrEnrollment)
	}
	record, err := writer.WriteEnrollment(
		ctx, plan.Receipt.ProjectID, plan.PolicyDigest, plan.ProfileDigest,
	)
	if err != nil {
		return fmt.Errorf("write enrollment: %w", err)
	}
	if record.State != model.EnrollmentEnrolled ||
		record.PolicyDigest != plan.PolicyDigest ||
		record.ProfileDigest != plan.ProfileDigest {
		return fmt.Errorf("written enrollment: %w", ErrEnrollment)
	}
	return nil
}

func verifyChanges(changes []Change) error {
	for _, change := range changes {
		data, exists, err := readOptional(change.Path)
		if err != nil {
			return err
		}
		if exists != change.Existed || !bytes.Equal(data, change.Before) {
			return changedArtifact(change.Path, ErrChanged)
		}
	}
	return nil
}

func applyChange(change Change) error {
	if change.Remove {
		if err := os.Remove(change.Path); err != nil {
			return fmt.Errorf("remove artifact: %w", err)
		}
		return nil
	}
	return WriteAtomic(change.Path, change.After)
}
