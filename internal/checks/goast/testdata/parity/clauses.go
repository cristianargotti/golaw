package parity

// switchCases counts the statements of every case clause: 44 in all.
func switchCases(n int) int {
	switch n {
	case 0:
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
	case 1:
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
	default:
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
	return n
}

// selectClauses never descends a communication clause: 5 statements over
// 46 lines, reported as lines.
func selectClauses(a, b chan int) int {
	n := 0
	select {
	case v := <-a:
		n += v
		n += v
		n += v
		n += v
		n += v
		n += v
		n += v
		n += v
		n += v
		n += v
		n += v
		n += v
		n += v
		n += v
		n += v
		n += v
		n += v
		n += v
		n += v
		n += v
	case v := <-b:
		n -= v
		n -= v
		n -= v
		n -= v
		n -= v
		n -= v
		n -= v
		n -= v
		n -= v
		n -= v
		n -= v
		n -= v
		n -= v
		n -= v
		n -= v
		n -= v
		n -= v
		n -= v
		n -= v
		n -= v
	}
	return n
}
