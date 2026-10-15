package goast_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
)

const mountSource = `package app

import (
	"encoding/json"
	"strconv"
)

func f(raw string, resolve func() (string, error), use func(string)) int {
	if dir, err := resolve(); err == nil {
		use(dir)
	}
	if dir, err := resolve(); err == nil { // fallback: the working directory serves when the root is unset
		use(dir)
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return n
	}
	var v int
	if err := json.Unmarshal([]byte(raw), &v); err == nil {
		return v
	}
	if dir, err := resolve(); err != nil {
		use(dir)
	}
	var dir string
	var err error
	if dir, err = resolve(); err == nil {
		use(dir)
	}
	if x := raw; x == "" {
		use(x)
	}
	if n, err := len(raw), error(nil); err == nil {
		return n
	}
	if err := failure; err == nil {
		return 1
	}
	if raw == "" {
		return 2
	}
	return 0
}
`

func TestInitIfErrNil(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.InitIfErrNil(), map[string]string{
		"internal/app/mount.go":      mountSource,
		"internal/app/mount_test.go": mountSource,
	})
	expectCodes(t, res, "init-if-err-nil", "init-if-err-nil")
	expectLines(t, res, 9, 36)
	if !strings.HasPrefix(res.Findings[0].Message, "an init-if keeps the happy path and drops the error") {
		t.Errorf("message = %q", res.Findings[0].Message)
	}
}
