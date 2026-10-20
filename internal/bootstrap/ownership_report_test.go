package bootstrap_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
)

func TestOwnershipReportsProtectedPathsAndReadLimits(t *testing.T) {
	t.Parallel()
	const notice = "CODEOWNERS line 2: unsupported negation: !/docs/"
	for _, tc := range []struct{ language, protected, unsupported string }{
		{"", "Destino protegido por CODEOWNERS, preservado: ", "Limite de leitura, sem atribuição de propriedade: "},
		{"pt-BR", "Destino protegido por CODEOWNERS, preservado: ", "Limite de leitura, sem atribuição de propriedade: "},
		{"en", "Destination protected by CODEOWNERS, preserved: ", "Read limit, no ownership assigned: "},
	} {
		t.Run(tc.language, func(t *testing.T) {
			t.Parallel()
			plan := bootstrap.TemplatePlan{
				Language: tc.language,
				Ownership: bootstrap.Ownership{
					Allowed: []string{"allowed.txt"}, Protected: []string{"protected.txt"}, Unsupported: []string{notice},
				},
			}
			var out bytes.Buffer
			if err := bootstrap.WriteTemplateDiff(&out, plan); err != nil {
				t.Fatal(err)
			}
			want := tc.protected + "protected.txt.\n" + tc.unsupported + notice + "\n"
			if out.String() != want {
				t.Fatalf("ownership report = %q, want %q", out.String(), want)
			}
		})
	}
}

func TestOwnershipReportFailureStopsBeforeLaterContent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, want string
		calls      int
	}{
		{"protected notice", "", 1},
		{"unsupported notice", "Destino protegido por CODEOWNERS, preservado: protected.txt.\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plan := bootstrap.TemplatePlan{
				Ownership: bootstrap.Ownership{
					Protected: []string{"protected.txt"}, Unsupported: []string{"unsupported pattern"},
				},
				Changes: []install.Change{{Path: "later.txt", After: []byte("new\n")}},
			}
			writer := &templateMutationWriter{fail: true, calls: tc.calls}
			var out bytes.Buffer
			err := bootstrap.WriteTemplateDiff(io.MultiWriter(writer, &out), plan)
			if !errors.Is(err, io.ErrClosedPipe) || err.Error() != "bootstrap: write ownership notice: "+io.ErrClosedPipe.Error() {
				t.Fatalf("ownership report error = %v", err)
			}
			if out.String() != tc.want || writer.calls != 2 {
				t.Fatalf("content after failure = %q, writes = %d", out.String(), writer.calls)
			}
		})
	}
}
