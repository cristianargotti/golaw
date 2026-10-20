package main

import "testing"

func TestM2TimingPreparation(t *testing.T) {
	t.Parallel()
	for _, rung := range []string{"pre-commit", "commit-msg", "pre-push"} {
		for _, mode := range []struct {
			name string
			fail bool
		}{
			{"answer", false},
			{"corrupt", true},
			{"broken", true},
		} {
			s := m2TestSuite(t)
			script := m2TimingScript(t, s, mode.name)
			s.execute = script.Execute
			c, err := s.timingM2(t.Context(), rung, t.TempDir())
			if (err != nil) != mode.fail {
				t.Fatalf("%s %s: error=%v", rung, mode.name, err)
			}
			if !mode.fail && (c.program == "" || c.dir == "") {
				t.Fatalf("%s: incomplete installed invocation", rung)
			}
			if script.Calls == 0 {
				t.Fatal("timing preparation ignored executor")
			}
		}
	}
}

func TestM2TimingMetadata(t *testing.T) {
	t.Parallel()
	for _, mode := range []struct {
		name string
		fail bool
	}{
		{"answer", false},
		{"corrupt", true},
		{"broken", true},
	} {
		s := m2TestSuite(t)
		script := m2TimingScript(t, s, mode.name)
		s.execute = script.Execute
		metadata, err := s.timingMetadataM2(t.Context())
		if (err != nil) != mode.fail {
			t.Fatalf("%s: error=%v", mode.name, err)
		}
		if !mode.fail && (metadata["revision"] == "" || metadata["database"] != "fixture") {
			t.Fatalf("%s: metadata=%v", mode.name, metadata)
		}
		if script.Calls == 0 {
			t.Fatal("timing metadata ignored executor")
		}
	}
}
