package main

import (
	"fmt"
	"io"
	"runtime"
	"strings"
)

type rowEvidence struct {
	Agent   string
	Version string
	Binary  string
}

func reportEvidence(out io.Writer, rows []row, version, digest string, evidence map[string]rowEvidence) bool {
	if _, err := fmt.Fprintln(out,
		"CATEGORY\tAGENT\tOS\tVERSION\tBINARY\tSTATUS\tREQUIRED\tCASE\tDETAIL"); err != nil {
		return false
	}
	var failing []string
	for _, item := range rows {
		if blocks(item) {
			failing = append(failing, failure(item))
		}
		identity, found := evidence[item.category+"/"+item.name]
		if !found {
			identity = rowEvidence{Agent: agentClaude, Version: version, Binary: digest}
		}
		if !writeM2Row(out, item, identity) {
			return false
		}
	}
	if len(failing) == 0 {
		return true
	}
	_, _ = fmt.Fprintf(out, "rows that fail the run: %s\n", strings.Join(failing, ", ")) //nolint:errcheck // The run fails whatever the write answers.
	return false
}

func writeM2Row(out io.Writer, item row, identity rowEvidence) bool {
	need := "optional"
	if item.required {
		need = "required"
	}
	detail := strings.Join(strings.Fields(item.detail), " ")
	_, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		item.category, identity.Agent, runtime.GOOS, identity.Version, identity.Binary,
		item.status, need, item.name, detail)
	return err == nil
}
