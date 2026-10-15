package report_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/report"
)

// failAfter is a writer that fails once a number of writes succeeded.
type failAfter struct {
	ok int
}

func (w *failAfter) Write(p []byte) (int, error) {
	if w.ok == 0 {
		return 0, model.ErrUnreadable
	}
	w.ok--
	return len(p), nil
}

func TestWriteText(t *testing.T) {
	t.Parallel()
	r, d := run(t)
	var out bytes.Buffer
	if err := report.WriteText(&out, report.New(r, d, identity())); err != nil {
		t.Fatal(err)
	}
	want := "-:0:0 note shell.push git push observed [rules/4.1@1]\n" +
		"docs/a.md:3:7 deny text.dashes em dash in text [rules/4.1@1]\n" +
		"goast.goleak unmeasured external-evidence is not available in scan mode\n" +
		"text.todo unmeasured read a.go: path unreadable\n" +
		"verdict fail: 2 findings (1 enforced, 1 notes), 1 excluded, 2 unmeasured\n"
	if out.String() != want {
		t.Errorf("text report =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestWritersReportTheFailedWrite(t *testing.T) {
	t.Parallel()
	r, d := run(t)
	doc := report.New(r, d, identity())
	cases := map[int]string{0: "write finding", 2: "write status", 4: "write verdict"}
	for ok, want := range cases {
		err := report.WriteText(&failAfter{ok: ok}, doc)
		if err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("after %d writes: %v, want %s", ok, err, want)
		}
	}
	if err := report.WriteJSON(&failAfter{}, doc); err == nil || !strings.HasPrefix(err.Error(), "write report") {
		t.Errorf("json: %v", err)
	}
}

func TestWriteJSONIsDeterministic(t *testing.T) {
	t.Parallel()
	r, d := run(t)
	doc := report.New(r, d, identity())
	var first, second bytes.Buffer
	if err := report.WriteJSON(&first, doc); err != nil {
		t.Fatal(err)
	}
	if err := report.WriteJSON(&second, doc); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() || !strings.HasSuffix(first.String(), "}\n") {
		t.Error("two renderings differ or the document is unterminated")
	}
	var back report.Document
	if err := json.Unmarshal(first.Bytes(), &back); err != nil {
		t.Fatal(err)
	}
	if back.Counts != doc.Counts || back.Findings[0].Severity != model.SeverityNote || back.Findings[1].Severity != model.SeverityDeny {
		t.Errorf("decoded = %+v", back)
	}
	keys := []string{`"schema": 1`, `"kit_version": "0.1.0"`, `"policy_digest"`, `"ruleset_digest"`, `"snapshot"`, `"findings"`, `"checks"`, `"counts"`}
	for _, key := range keys {
		if !strings.Contains(first.String(), key) {
			t.Errorf("document lacks %s", key)
		}
	}
}
