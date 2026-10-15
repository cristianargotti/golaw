package fn

import "testing"

func TestWalk(t *testing.T) {
	n := 0
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	if walk(0) != n {
		t.Fatal(n)
	}
}
