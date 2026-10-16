package goast_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
)

const moneySource = `package app

type Cart struct{ Prices []int }

func (c Cart) Total() int {
	sum := 0
	for _, p := range c.Prices {
		sum += p
	}
	return sum
}

func (c Cart) Sum() int {
	sum := 0
	for _, p := range c.Prices {
		sum += p
	}
	return sum
}

func (c *Cart) PtrSum() int {
	sum := 0
	for _, p := range c.Prices {
		sum += p
	}
	return sum
}

func Free(c Cart) int {
	sum := 0
	for _, p := range c.Prices {
		sum += p
	}
	return sum
}

func Alias(c Cart) int {
	sum := 0
	for _, p := range c.Prices {
		sum += p
	}
	return sum
}

func One(c Cart) int { return len(c.Prices) }

func Two(c Cart) int { return len(c.Prices) }

func First[T any](xs []T) T {
	var zero T
	return zero
}

func Second[T any](xs []T) T {
	var zero T
	return zero
}

func external()
`

func TestSonarIdenticalFunctions(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.SonarIdenticalFunctions(), map[string]string{"internal/app/money.go": moneySource})
	expectCodes(t, res, "identical-functions", "identical-functions", "identical-functions")
	expectLines(t, res, 13, 37, 54)
	want := "function Sum has the receiver, the parameters and the body of Total: keep one"
	if res.Findings[0].Message != want {
		t.Errorf("message = %q", res.Findings[0].Message)
	}
}

// layoutSource repeats two bodies under other layouts: blank lines between
// and inside statements, a literal and a call split over lines, comments,
// and a var declaration in parentheses; none makes another function.
const layoutSource = `package app

func Base(xs []int) []int {
	out := []int{1, 2}
	for _, x := range xs {
		out = append(out, x)
	}
	return out
}

func Spaced(xs []int) []int {
	out := []int{1, 2}

	for _, x := range xs {

		out = append(out, x)
	}
	return out
}

func Split(xs []int) []int {
	out := []int{
		1,
		2,
	}
	for _, x := range xs {
		out = append(
			out,
			x,
		)
	}
	return out
}

func Commented(xs []int) []int {
	// out starts with the two defaults.
	out := []int{1, 2} // the defaults
	for _, x := range xs {
		/* each one */ out = append(out, x)
	}
	return out
}

func Grouped() int {
	var (
		a = 1
	)
	return a
}

func Plain() int {
	// a is one.
	var a = 1
	return a
}
`

func TestSonarIdenticalFunctionsIgnoreTheLayout(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.SonarIdenticalFunctions(), map[string]string{"internal/app/layout.go": layoutSource})
	expectCodes(t, res, "identical-functions", "identical-functions", "identical-functions", "identical-functions")
	expectLines(t, res, 11, 21, 35, 51)
	want := "function Plain has the receiver, the parameters and the body of Grouped: keep one"
	if res.Findings[3].Message != want {
		t.Errorf("message = %q", res.Findings[3].Message)
	}
}

// nearSource pairs functions of one signature whose bodies differ in one
// name, literal or token, or in which optional part is present, what a
// walk without positions could lose: no two of them are identical.
const nearSource = `package app

func A1(s []int, n int, c chan int, f fn) int { n += s[0]; return n }
func A2(s []int, n int, c chan int, f fn) int { n -= s[0]; return n }
func B1(s []int, n int, c chan int, f fn) int { ok := n < s[0]; return b(ok) }
func B2(s []int, n int, c chan int, f fn) int { ok := n > s[0]; return b(ok) }
func C1(s []int, n int, c chan int, f fn) int { n = f(s...); return n }
func C2(s []int, n int, c chan int, f fn) int { n = f(s); return n }
func D1(s []int, n int, c chan int, f fn) int { n++; return n }
func D2(s []int, n int, c chan int, f fn) int { n--; return n }
func E1(s []int, n int, c chan int, f fn) int { for range s { break }; return n }
func E2(s []int, n int, c chan int, f fn) int { for range s { continue }; return n }
func F1(s []int, n int, c chan int, f fn) int { n = -n; return n }
func F2(s []int, n int, c chan int, f fn) int { n = ^n; return n }
func G1(s []int, n int, c chan int, f fn) int { for n = range s {}; return n }
func G2(s []int, n int, c chan int, f fn) int { for n := range s {}; return n }
func H1(s []int, n int, c chan int, f fn) int { var k = 1; return n + k }
func H2(s []int, n int, c chan int, f fn) int { const k = 1; return n + k }
func I1(s []int, n int, c chan int, f fn) int { var d chan<- int; return use(d) }
func I2(s []int, n int, c chan int, f fn) int { var d <-chan int; return use(d) }
func J1(s []int, n int, c chan int, f fn) int { type t = int; return int(t(n)) }
func J2(s []int, n int, c chan int, f fn) int { type t int; return int(t(n)) }
func K1(s []int, n int, c chan int, f fn) int { s = s[n:]; return len(s) }
func K2(s []int, n int, c chan int, f fn) int { s = s[:n]; return len(s) }
func L1(s []int, n int, c chan int, f fn) int { for f(); ; { break }; return n }
func L2(s []int, n int, c chan int, f fn) int { for ; ; f() { break }; return n }
func M1(s []int, n int, c chan int, f fn) int { _ = [][]int{{n, n}}; return n }
func M2(s []int, n int, c chan int, f fn) int { _ = [][]int{n{n}}; return n }
func N1(s []int, n int, c chan int, f fn) int { select { case c <- n: n++ }; return n }
func N2(s []int, n int, c chan int, f fn) int { select { default: c <- n; n++ }; return n }
func O1(s []int, n int, c chan int, f fn) int { n, s[0] = s[1], n; return n }
func O2(s []int, n int, c chan int, f fn) int { n, s[0], s[1] = n; return n }
func P1(s []int, n int, c chan int, f fn) int { var a, b = n; return a }
func P2(s []int, n int, c chan int, f fn) int { var a b = n; return a }
func Q1(s []int, n int, c chan int, f fn) int { n = len(s); return n }
func Q2(s []int, n int, c chan int, f fn) int { n = cap(s); return n }
func R1(s []int, n int, c chan int, f fn) int { n = 1; return n }
func R2(s []int, n int, c chan int, f fn) int { n = 2; return n }
`

func TestSonarIdenticalFunctionsKeepEveryToken(t *testing.T) {
	t.Parallel()
	expectCodes(t, scan(t, goast.SonarIdenticalFunctions(), map[string]string{"internal/app/near.go": nearSource}))
}

// familyChecks is the size of the family once its last check is listed.
const familyChecks = 17

func TestFamilyIsComplete(t *testing.T) {
	t.Parallel()
	if got := len(goast.Checks()); got != familyChecks {
		t.Fatalf("the family lists %d checks, want %d", got, familyChecks)
	}
}
