package parity

// assignedLiteral counts the statements of a function literal assigned on
// the right-hand side: 45 in all, reported as statements.
func assignedLiteral(n int) int {
	f := func() {
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
	}
	f()
	f()
	f()
	f()
	f()
	f()
	f()
	f()
	f()
	f()
	f()
	f()
	f()
	return n
}

// calledLiteral passes a function literal to a call, which funlen never
// descends: 11 statements over 42 lines, reported as lines.
func calledLiteral(n int) int {
	run(func() {
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
		n++
	})
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	return n
}

func run(f func()) { f() }
