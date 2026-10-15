package goast_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
	"github.com/cristianargotti/golaw/internal/model"
)

const workerSource = `package jobs

import "sync"

func Start(work func()) {
	var wg sync.WaitGroup
	wg.Go(work)
	go work()
	wg.Wait()
}
`

const otherSource = "package jobs\n\nfunc Other(work func()) {\n\tgo work()\n}\n"

const verifiedSource = "package jobs\n\nimport (\n\t\"testing\"\n\n\t\"go.uber.org/goleak\"\n)\n\nfunc TestMain(m *testing.M) {\n\tgoleak.VerifyTestMain(m)\n}\n"

func TestGoleak(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.Goleak(), map[string]string{
		"internal/jobs/worker.go":      workerSource,
		"internal/jobs/other.go":       otherSource,
		"internal/jobs/worker_test.go": "package jobs\n",
		"internal/calm/calm.go":        "package calm\n\nfunc Go() {}\n\nfunc f() { Go() }\n",
	})
	expectCodes(t, res, "goleak")
	expectLines(t, res, 4)
	want := "package internal/jobs starts goroutines and no test of the package verifies with goleak"
	if res.Findings[0].Path != "internal/jobs/other.go" || res.Findings[0].Message != want {
		t.Errorf("finding = %+v", res.Findings[0])
	}
	verified := scan(t, goast.Goleak(), map[string]string{
		"internal/jobs/worker.go":    workerSource,
		"internal/jobs/leak_test.go": verifiedSource,
	})
	expectCodes(t, verified)
}

func TestGoleakReadsTheTests(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("x", model.MaxReadBytes+1)
	res := scan(t, goast.Goleak(), map[string]string{"internal/jobs/worker.go": workerSource, "internal/jobs/zzz_test.go": big})
	expectUnmeasured(t, res, "read internal/jobs/zzz_test.go")
	res = scan(t, goast.Goleak(), map[string]string{"internal/jobs/worker.go": workerSource, "internal/jobs/zzz_test.go": "package jobs\n\nimport (\n"})
	expectUnmeasured(t, res, "parse internal/jobs/zzz_test.go")
}
