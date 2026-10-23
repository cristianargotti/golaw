package main

import (
	"encoding/json"
	"fmt"
)

// ciRun is the part of one run of the runs endpoint the gate reads.
type ciRun struct {
	HeadSHA    string `json:"head_sha"`
	HeadBranch string `json:"head_branch"`
	Event      string `json:"event"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	RunNumber  int    `json:"run_number"`
	RunAttempt int    `json:"run_attempt"`
}

// runsPage is one answer of the runs endpoint; a missing list is a file of
// another kind, never an empty history.
type runsPage struct {
	Runs *[]ciRun `json:"workflow_runs"`
}

// Decide answers whether the commit may be released and the line that
// says why, the judgment the lane's former jq query made: among the runs
// of the commit on main by push, the last by run number and attempt must
// be completed with the conclusion success. A file that is not an answer
// of the endpoint is an error, so the gate never decides on what it
// cannot read.
func Decide(runs []byte, sha string) (bool, string, error) {
	var page runsPage
	if err := json.Unmarshal(runs, &page); err != nil {
		return false, "", fmt.Errorf("%w: %w", errRuns, err)
	}
	if page.Runs == nil {
		return false, "", fmt.Errorf("%w: the file holds no workflow_runs list", errRuns)
	}
	last, found := latest(*page.Runs, sha)
	switch {
	case !found:
		return false, fmt.Sprintf(lineNoRun, sha), nil
	case last.Status != "completed" || last.Conclusion != "success":
		return false, fmt.Sprintf(lineNotGreen, sha, last.RunNumber, last.RunAttempt, last.Status, conclusionOf(last)), nil
	}
	return true, fmt.Sprintf(lineGreen, sha, last.RunNumber, last.RunAttempt), nil
}

// latest is the last push run of the commit on main, by run number and
// then attempt, so a re-run decides over the run it repeats.
func latest(runs []ciRun, sha string) (ciRun, bool) {
	var last ciRun
	found := false
	for _, run := range runs {
		if run.HeadSHA != sha || run.HeadBranch != "main" || run.Event != "push" {
			continue
		}
		if !found || later(run, last) {
			last, found = run, true
		}
	}
	return last, found
}

func later(a, b ciRun) bool {
	if a.RunNumber != b.RunNumber {
		return a.RunNumber > b.RunNumber
	}
	return a.RunAttempt > b.RunAttempt
}

// conclusionOf names the conclusion of a run still running, which the
// endpoint answers as null.
func conclusionOf(run ciRun) string {
	if run.Conclusion == "" {
		return noConclusion
	}
	return run.Conclusion
}
