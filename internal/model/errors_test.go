package model_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestSentinelsAreDistinctAndNamed(t *testing.T) {
	t.Parallel()
	all := []error{
		model.ErrUsage, model.ErrInvalidSpec, model.ErrDuplicateCheck, model.ErrUnknownProof,
		model.ErrNotInSnapshot, model.ErrUnreadable, model.ErrTooLarge, model.ErrPolicy,
		model.ErrEnrollment, model.ErrDeadline, model.ErrCannotJudge,
	}
	for i, a := range all {
		if a == nil || a.Error() == "" {
			t.Fatalf("sentinel %d is nil or unnamed", i)
		}
		for j, b := range all {
			if i != j && errors.Is(a, b) {
				t.Errorf("sentinels %d and %d are not distinct: %v", i, j, a)
			}
		}
	}
}
