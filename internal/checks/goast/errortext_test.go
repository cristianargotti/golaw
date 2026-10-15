package goast_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
)

const syncSource = `package application

import (
	"errors"
	"fmt"
	"strings"
)

type row struct {
	Message string
}

type SyncError struct {
	Text string
}

type ErrSync struct {
	Text string
}

var global = errors.New("x").Error()

func f(err error, logger logs, repo repo) (string, error) {
	a := err.Error()
	r := row{Message: err.Error()}
	rows := []row{{err.Error()}}
	repo.Save(err.Error())
	msg := "sync: " + err.Error()
	if err.Error() == "boom" {
		return err.Error(), nil
	}
	logger.Warn("sync failed", "cause", err.Error())
	slog.Info("x", "cause", err.Error())
	t.Log(err.Error())
	_ = SyncError{Text: err.Error()}
	_ = ErrSync{Text: err.Error()}
	_ = strings.Contains(err.Error(), "x")
	cause := failures.Root(err).Error()
	_, _ = fmt.Errorf("x: %s", err.Error()), errors.Is(err, nil)
	err.Error()
	_ = err.Error
	return a + r.Message + rows[0].Message + msg + cause, nil
}
`

func TestErrorTextAsData(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.ErrorTextAsData(), map[string]string{
		"internal/application/sync.go":          syncSource,
		"internal/application/errors.go":        syncSource,
		"internal/domain/sync.go":               syncSource,
		"internal/adapters/outbound/pg/sync.go": "package pg\n\nfunc g(err error) string {\n\treturn err.Error()\n}\n",
	})
	expectCodes(t, res, "error-text-as-data", "error-text-as-data", "error-text-as-data", "error-text-as-data",
		"error-text-as-data", "error-text-as-data", "error-text-as-data", "error-text-as-data")
	expectLines(t, res, 4, 21, 24, 25, 26, 27, 28, 30)
	stored := "error text is stored as data and carries the wrap chain: keep the root cause or a typed message"
	passed := "error text is passed as data and carries the wrap chain: keep the root cause or a typed message"
	returned := "error text is returned as data and carries the wrap chain: keep the root cause or a typed message"
	if res.Findings[0].Message != returned || res.Findings[2].Message != stored || res.Findings[5].Message != passed {
		t.Errorf("messages = %+v", res.Findings)
	}
}
