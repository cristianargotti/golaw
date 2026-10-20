package main

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestM2MeasurementInvocationAndEnvironment(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.m2.Live = true
	s.env = append(s.env, "golaw_m2_measure=0", "GOLAW_M2_MEASURE=ignored", "GOLAW_M2_LIVE_DB=1", "KEEP=literal")
	original := slices.Clone(s.env)
	calls := 0
	s.execute = func(_ context.Context, c command) result {
		calls++
		want := []string{"test", "-race", "-timeout=45m", "-run", "^TestM2Measurements$", "-count=1", "-v", "./tools/acceptance/"}
		if c.program != s.goTool || c.dir != s.root || !slices.Equal(c.args, want) || c.input != "" {
			t.Fatalf("measurement invocation=%+v", c)
		}
		if c.timeout != 50*time.Minute {
			t.Fatalf("measurement executor timeout=%s, want 50m", c.timeout)
		}
		m2RequireMeasurementEnvironment(t, c.env)
		return result{stdout: "    m2_measurement_internal_test.go:47: M2_MEASUREMENTS_COMPLETE\n"}
	}
	item := s.measurementM2(t.Context())
	if calls != 1 || item.status != statusPass || !item.required || !slices.Equal(original, s.env) {
		t.Fatalf("measurement calls=%d row=%+v parent environment preserved=%v", calls, item, slices.Equal(original, s.env))
	}
}

func m2RequireMeasurementEnvironment(t *testing.T, env []string) {
	t.Helper()
	count := 0
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "GOLAW_M2_MEASURE") {
			count++
			if value != "1" {
				t.Fatalf("measurement retained inherited disable value %q", value)
			}
		}
	}
	if count != 1 || environmentValue(env, "GOLAW_M2_LIVE_DB") != "1" || environmentValue(env, "KEEP") != "literal" {
		t.Fatalf("measurement environment lost its opt-in or inherited values: %q", env)
	}
}

func TestM2MeasurementCompletionIsAnExactLine(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		got  result
		pass bool
	}{
		{"plain", result{stdout: "M2_MEASUREMENTS_COMPLETE\n"}, true},
		{"Go test log", result{stdout: "=== RUN TestM2Measurements\n    m2_measurement_internal_test.go:47: M2_MEASUREMENTS_COMPLETE\n"}, true},
		{"CRLF", result{stdout: "M2_MEASUREMENTS_COMPLETE\r\n"}, true},
		{"stderr only", result{stderr: "M2_MEASUREMENTS_COMPLETE\n"}, false},
		{"prefix", result{stdout: "not M2_MEASUREMENTS_COMPLETE\n"}, false},
		{"suffix", result{stdout: "M2_MEASUREMENTS_COMPLETE_not_really\n"}, false},
		{"log suffix", result{stdout: "    m2_measurement_internal_test.go:47: M2_MEASUREMENTS_COMPLETE pending\n"}, false},
		{"non-numeric log", result{stdout: "    m2_measurement_internal_test.go:unknown: M2_MEASUREMENTS_COMPLETE\n"}, false},
		{"arbitrary prefix", result{stdout: "failure: M2_MEASUREMENTS_COMPLETE\n"}, false},
		{"split channels", result{stdout: "M2_MEASUREMENTS_", stderr: "COMPLETE\n"}, false},
		{"marker with failure", result{stdout: "M2_MEASUREMENTS_COMPLETE\n", code: 2}, false},
		{"marker with process error", result{stdout: "M2_MEASUREMENTS_COMPLETE\n", err: ErrAcceptance}, false},
	} {
		s := m2TestSuite(t)
		s.execute = func(context.Context, command) result { return tt.got }
		item := s.measurementM2(t.Context())
		if (item.status == statusPass) != tt.pass || !item.required {
			t.Fatalf("%s: row=%+v, pass=%v", tt.name, item, tt.pass)
		}
	}
}
