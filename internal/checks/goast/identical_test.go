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

// familyChecks is the size of the family once its last check is listed.
const familyChecks = 17

func TestFamilyIsComplete(t *testing.T) {
	t.Parallel()
	if got := len(goast.Checks()); got != familyChecks {
		t.Fatalf("the family lists %d checks, want %d", got, familyChecks)
	}
}
