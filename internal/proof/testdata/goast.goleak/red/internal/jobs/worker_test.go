package jobs

import "testing"

func TestStart(t *testing.T) {
	ran := false
	Start(func() { ran = true }).Wait()
	if !ran {
		t.Fatal("the worker did not run")
	}
}
