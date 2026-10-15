package goast_test

import (
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
	"github.com/cristianargotti/golaw/internal/model"
)

// adapterTree holds 13 non-test files under inbound/http, 12 under
// outbound/pg, a test file and a sub-package.
func adapterTree() map[string]string {
	files := map[string]string{
		"internal/adapters/inbound/http/f14_test.go": "package http\n",
		"internal/adapters/inbound/http/ctx/a.go":    "package ctx\n",
	}
	for i := 1; i <= 13; i++ {
		files[fmt.Sprintf("internal/adapters/inbound/http/f%02d.go", i)] = "package http\n"
	}
	for i := 1; i <= 12; i++ {
		files[fmt.Sprintf("internal/adapters/outbound/pg/f%02d.go", i)] = "package pg\n"
	}
	return files
}

func TestAdapterPackageSize(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.AdapterPackageSize(), adapterTree())
	expectCodes(t, res, "adapter-package-size")
	expectLines(t, res, 0)
	f := res.Findings[0]
	want := "13 non-test files in one adapter package, the limit is 12: the root keeps the shared plumbing and each context lives in its own package"
	if f.Path != "internal/adapters/inbound/http" || f.Message != want {
		t.Errorf("finding = %+v", f)
	}
}

func TestAdapterPackageSizeOption(t *testing.T) {
	t.Parallel()
	c := goast.AdapterPackageSize()
	in := input(c, adapterTree())
	in.Options = model.Options{"max_files": model.IntValue(2)}
	res, err := c.Run(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	expectCodes(t, res, "adapter-package-size", "adapter-package-size")
	in.Options = model.Options{}
	res, err = c.Run(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	expectCodes(t, res, "adapter-package-size")
}

func TestAdapterPackageSizeNeedsAdapters(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.AdapterPackageSize(), map[string]string{"internal/a.go": "package a\n"})
	expectNotApplicable(t, res, "internal/adapters is absent")
}
