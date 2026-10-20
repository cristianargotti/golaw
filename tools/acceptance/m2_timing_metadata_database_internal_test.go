package main

import (
	"context"
	"strings"
	"testing"
)

func TestM2TimingMetadataCapturesReportedLiveDatabase(t *testing.T) {
	t.Parallel()
	s := m2MetadataSuite(t)
	s.m2.Live = true
	m2MetadataDatabaseAnswer(s, "DB: https://database.example.test/vuln\nDB updated: 2026-10-18T20:21:22Z\n")
	got, err := s.timingMetadataM2(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got["database_source"] != "https://database.example.test/vuln" ||
		got["database_updated"] != "2026-10-18T20:21:22Z" {
		t.Fatalf("reported database evidence was lost: %v", got)
	}
}

func TestM2TimingMetadataCapturesScannerTimestamp(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		reported, expected string
	}{
		{"2026-10-18 20:21:22 +0000 UTC", "2026-10-18T20:21:22Z"},
		{"2026-10-18 20:21:22.123456789 +0000 UTC", "2026-10-18T20:21:22.123456789Z"},
		{"2026-10-18 15:21:22 -0500 -0500", "2026-10-18T15:21:22-05:00"},
	} {
		s := m2MetadataSuite(t)
		s.m2.Live = true
		m2MetadataDatabaseAnswer(s, "DB: https://vuln.go.dev\nDB updated: "+tc.reported+"\n")
		got, err := s.timingMetadataM2(t.Context())
		if err != nil || got["database_updated"] != tc.expected {
			t.Fatalf("scanner timestamp %q: got=%v error=%v", tc.reported, got, err)
		}
	}
}

func TestM2TimingMetadataOmitsInvalidDatabaseEvidence(t *testing.T) {
	t.Parallel()
	for _, answer := range []string{
		"DB: /var/lib/database\nDB updated: not a timestamp\n",
		"DB: file:///var/lib/database\nDB updated: 2026-99-99T20:21:22Z\n",
		"DB: https://person:secret@database.example.test\nDB updated: now\n",
		"DB: https://database.example.test?token=secret\nDB updated: 2026-10-18\n",
		"DB: https://database.example.test#private\nDB updated: 2026-10-18T20:21:22\n",
		"DB: https://\nDB updated: /var/lib/date\n",
		"DB: https://:\n",
		"DB: %\n",
	} {
		s := m2MetadataSuite(t)
		s.m2.Live = true
		m2MetadataDatabaseAnswer(s, answer)
		got, err := s.timingMetadataM2(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if got["database_source"] != "https://vuln.go.dev" || got["database_updated"] != "" {
			t.Fatalf("invalid evidence was published: %v", got)
		}
	}
}

func TestM2TimingMetadataKeepsFixtureProvenance(t *testing.T) {
	t.Parallel()
	s := m2MetadataSuite(t)
	m2MetadataDatabaseAnswer(s, "DB: https://database.example.test\nDB updated: 2026-10-18T20:21:22Z\n")
	got, err := s.timingMetadataM2(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got["database_source"] != "tools/acceptance/testdata/m2/empty-db" || got["database_updated"] != "" {
		t.Fatalf("live probe replaced fixture provenance: %v", got)
	}
}

func TestM2TimingMetadataRejectsFailedDatabaseProbe(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"execution", "exit", "empty", "invalid"} {
		s := m2MetadataSuite(t)
		s.m2.Live = true
		fixture, calls := s.execute, 0
		s.execute = func(ctx context.Context, c command) result {
			if m2MetadataVersionProbe(c, m2Scanner) {
				calls++
				if calls == 2 {
					return m2InvalidVersionResult(m2Scanner, mode)
				}
			}
			return fixture(ctx, c)
		}
		if _, err := s.timingMetadataM2(t.Context()); err == nil || calls != 2 {
			t.Fatalf("%s: database probe calls=%d error=%v", mode, calls, err)
		}
	}
}

func m2MetadataDatabaseAnswer(s *suite, answer string) {
	fixture := s.execute
	s.execute = func(ctx context.Context, c command) result {
		got := fixture(ctx, c)
		if m2MetadataVersionProbe(c, m2Scanner) {
			got.stdout = strings.TrimSpace(got.stdout) + "\n" + answer
		}
		return got
	}
}
