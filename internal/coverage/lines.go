package coverage

import "fmt"

// Lines keeps floor decisions intact while rounding only the displayed percentages.
func (r Report) Lines() []string {
	lines := make([]string, 0, 1+len(r.Failures)+len(r.Skipped))
	lines = append(lines, measureLine(r.Total))
	for _, failure := range r.Failures {
		if failure.Statements == 0 {
			lines = append(lines, "file missing from the profile: "+failure.Name)
			continue
		}
		lines = append(lines, "below the floor: "+measureLine(failure))
	}
	for _, skipped := range r.Skipped {
		lines = append(lines, "under the statement minimum: "+measureLine(skipped))
	}
	return lines
}

func measureLine(measure Measure) string {
	return fmt.Sprintf("%s: %.1f%% (%d/%d)", measure.Name, measure.Percent(), measure.Covered, measure.Statements)
}
