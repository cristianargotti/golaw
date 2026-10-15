package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/model"
)

// WriteText prints the findings one per line as path:line:column severity
// check-id message [rule], then every unmeasured check, then the verdict
// line with the counts. An event finding without a path prints a dash.
func WriteText(w io.Writer, doc Document) error {
	for _, f := range doc.Findings {
		if _, err := fmt.Fprintf(w, "%s %s %s %s [%s]\n", location(f), f.Severity, f.Check, f.Message, f.Rule); err != nil {
			return fmt.Errorf("write finding: %w", err)
		}
	}
	for _, s := range doc.Checks {
		if s.Status != model.StatusUnmeasured.String() {
			continue
		}
		if _, err := fmt.Fprintf(w, "%s unmeasured %s\n", s.ID, s.Reason); err != nil {
			return fmt.Errorf("write status: %w", err)
		}
	}
	c := doc.Counts
	_, err := fmt.Fprintf(w, "verdict %s: %d findings (%d enforced, %d notes), %d excluded, %d unmeasured\n",
		doc.Verdict, c.Findings, c.Enforced, c.Notes, c.Excluded, c.Unmeasured)
	if err != nil {
		return fmt.Errorf("write verdict: %w", err)
	}
	return nil
}

// location renders path:line:column, with a dash for a finding that has no
// path.
func location(f model.Finding) string {
	path := f.Path
	if path == "" {
		path = "-"
	}
	return fmt.Sprintf("%s:%d:%d", path, f.Line, f.Column)
}

// WriteJSON prints the document indented, keys in declaration order, with a
// final newline.
func WriteJSON(w io.Writer, doc Document) error {
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	if _, err := w.Write(append(body, '\n')); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}
