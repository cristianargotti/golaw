package jobs

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestStart(t *testing.T) {
	ran := false
	Start(func() { ran = true }).Wait()
	if !ran {
		t.Fatal("the worker did not run")
	}
}
