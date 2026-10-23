package doctor

// assess is every M1 line of a snapshot in the order of assessments, the
// lines claudeReport prints without the frame of the agent, so a test reads
// each diagnostic of a collection apart from the others.
func assess(snapshot Snapshot) Report {
	report := Report{Schema: 1, Lines: []Line{}}
	for _, item := range assessments() {
		report.Lines = append(report.Lines, item.run(snapshot)...)
	}
	return report
}
