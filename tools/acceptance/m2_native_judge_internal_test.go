package main

import "testing"

func TestM2NativeJudgmentUsesProducerStatuses(t *testing.T) {
	t.Parallel()
	const finding = "folder with space/note.txt:2:1 fail text.dashes forbidden dash [rules/4.1@1]\n"
	const clean = "verdict clean: 0 findings (0 enforced, 0 notes), 18 excluded, 0 unmeasured\n"
	const unmeasured = "golaw pre-push: main: govulncheck could not judge it, the pipeline judges it: exit status 2\n"
	for _, tc := range []struct {
		name     string
		got      result
		red      bool
		measured bool
		pass     bool
	}{
		{"silent", result{}, false, true, true},
		{"zero unmeasured", result{stdout: clean}, false, true, true},
		{"ordinary mention", result{stdout: "details: the word unmeasured is not a check status\n"}, false, true, true},
		{"note", result{stdout: "-:0:0 note text.dashes context [rules/4.1@1]\n"}, false, true, true},
		{"red stdout", result{code: 1, stdout: finding}, true, true, true},
		{"red stderr", result{code: 2, stderr: finding}, true, true, true},
		{"launcher refusal", result{code: 128, stdout: finding}, true, true, true},
		{"not refused", result{stdout: finding}, true, true, false},
		{"unrelated refusal", result{code: 1, stderr: "other check"}, true, true, false},
		{"near exit", result{code: 1}, false, true, false},
		{"near enforced", result{stdout: finding}, false, true, false},
		{"near enforced count", result{stdout: "verdict fail: 1 findings (1 enforced, 0 notes), 0 excluded, 0 unmeasured\n"}, false, true, false},
		{"positive unmeasured count", result{stdout: "verdict clean: 0 findings (0 enforced, 0 notes), 0 excluded, 1 unmeasured\n"}, false, true, false},
		{"invalid count", result{stdout: "verdict clean: invalid unmeasured\n"}, false, true, false},
		{"check status", result{stdout: "text.todo unmeasured source unreadable\n"}, false, true, false},
		{"scanner error", result{stdout: unmeasured}, false, true, false},
		{"red scanner error", result{code: 1, stderr: finding + unmeasured}, true, true, false},
		{"no measurement requested", result{stdout: unmeasured}, false, false, true},
		{"execution error", result{err: ErrAcceptance}, false, true, false},
		{"empty verdict", result{stdout: "verdict unmeasured\n"}, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := m2JudgeNative(tc.got, tc.red, "text.dashes", tc.measured); (err == nil) != tc.pass {
				t.Fatalf("pass=%t error=%v", tc.pass, err)
			}
		})
	}
}
