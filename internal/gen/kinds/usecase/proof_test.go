package usecase_test

import (
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/goldentest"
)

func TestTheProofOfTheUsecaseKind(t *testing.T) {
	t.Parallel()
	goldentest.Prove(t, filepath.Join("testdata", "proof.yml"), goldentest.Laws())
}
