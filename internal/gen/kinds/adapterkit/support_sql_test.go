package adapterkit_test

import (
	"bytes"
	"testing"
)

const (
	driverFile = "internal/gensupport/sqltest/zz_generated_driver.go"
	driverTest = "internal/gensupport/sqltest/zz_generated_driver_internal_test.go"
	driverErrs = "internal/gensupport/sqltest/zz_generated_errors.go"
)

func TestSupportRendersTheScriptedDriver(t *testing.T) {
	t.Parallel()
	files := supportFiles(t)
	for _, p := range []string{driverFile, driverTest, driverErrs} {
		content, ok := files[p]
		if !ok {
			t.Errorf("support lacks %s", p)
			continue
		}
		requireLaws(t, p, content)
	}
	for _, want := range []string{
		"type Step struct {", "Query    string", "Args     []driver.Value", "Columns  []string",
		"Rows     [][]driver.Value", "Err      error", "Affected int64",
		"func Open(tb testing.TB, steps ...Step) *sql.DB", "sql.OpenDB(",
	} {
		if !bytes.Contains(files[driverFile], []byte(want)) {
			t.Errorf("%s lacks %q", driverFile, want)
		}
	}
	if bytes.Contains(files[driverFile], []byte("sql.Register(")) {
		t.Errorf("%s registers a global driver name", driverFile)
	}
	if !bytes.Contains(files[driverErrs], []byte("var ErrUnscripted = errors.New(")) {
		t.Errorf("%s lacks ErrUnscripted", driverErrs)
	}
}

func TestSupportScriptedDriverPassesTheScan(t *testing.T) {
	t.Parallel()
	requireScanClean(t, supportFiles(t), driverFile, driverTest, driverErrs)
}

func TestSupportScriptedDriverCoversEveryLine(t *testing.T) {
	t.Parallel()
	requireCovered(t, supportFiles(t), "./internal/gensupport/sqltest/", "sqltest/zz_generated_driver.go")
}
