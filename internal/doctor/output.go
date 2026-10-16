package doctor

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteJSON keeps machine output independent of translated presentation.
func WriteJSON(writer io.Writer, report Report) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("write doctor JSON: %w", err)
	}
	return nil
}

// WriteText keeps diagnostics in English for consistent operational output.
func WriteText(writer io.Writer, report Report) error {
	if _, err := fmt.Fprintln(writer, "doctor"); err != nil {
		return fmt.Errorf("write doctor heading: %w", err)
	}
	for _, line := range report.Lines {
		if _, err := fmt.Fprintf(writer, "%s %s: %s [%s]\n",
			line.ID, line.Key, line.State, line.Code); err != nil {
			return fmt.Errorf("write doctor line: %w", err)
		}
	}
	return nil
}

// Print gives the command layer one explicit switch for its --json flag.
func Print(writer io.Writer, report Report, jsonOutput bool) error {
	if jsonOutput {
		return WriteJSON(writer, report)
	}
	return WriteText(writer, report)
}
