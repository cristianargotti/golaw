package main

import (
	"context"
	"fmt"
	"go/version"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runner"
)

const m2TimingCacheDescription = "cold: fresh GOCACHE, GOMODCACHE and GOLANGCI_LINT_CACHE per sample; " +
	"warm: reused after one untimed priming invocation; installed tools remain outside sample caches"

const m2TimingCachesKey = "caches"

func (s *suite) timingMetadataM2(ctx context.Context) (map[string]string, error) {
	if s.m2 == nil || !s.m2.Ready {
		return nil, fmt.Errorf("timing metadata requires verified tools: %w", ErrAcceptance)
	}
	revision, err := s.m2TimingRevision(ctx)
	if err != nil {
		return nil, err
	}
	files, err := s.m2TimingSource(ctx)
	if err != nil {
		return nil, err
	}
	metadata := map[string]string{
		"revision": revision, "tree": m2TimingTree(files), "samples": "30", m2TimingCachesKey: m2TimingCacheDescription,
		"database": "fixture", "database_source": "tools/acceptance/testdata/m2/empty-db",
	}
	if s.m2.Live {
		metadata["database"], metadata["database_source"] = "live", "https://vuln.go.dev"
	}
	if err := s.m2TimingVersions(ctx, metadata); err != nil {
		return nil, err
	}
	if s.m2.Live {
		if err := s.m2TimingDatabase(ctx, metadata); err != nil {
			return nil, err
		}
	}
	return metadata, nil
}

func (s *suite) m2TimingRevision(ctx context.Context) (string, error) {
	revision, err := s.m2ObserveGit(ctx, s.root, s.env, m2RevParse, m2Verify, m2Head)
	if err != nil {
		return "", err
	}
	if !regexp.MustCompile(`^([a-f0-9]{40}|[a-f0-9]{64})$`).MatchString(revision) {
		return "", fmt.Errorf("timing revision is not a Git object identifier: %w", ErrAcceptance)
	}
	return revision, nil
}

func m2TimingTree(files []m2ArchiveFile) string {
	ordered := slices.Clone(files)
	slices.SortFunc(ordered, func(a, b m2ArchiveFile) int { return strings.Compare(a.name, b.name) })
	var body []byte
	for _, file := range ordered {
		body = fmt.Appendf(body, "%d:%s:%d:", len(file.name), file.name, len(file.data))
		body = append(body, file.data...)
	}
	return install.Digest(body)
}

func (s *suite) m2TimingVersions(ctx context.Context, metadata map[string]string) error {
	for _, name := range []string{"go", m2GitCommand} {
		value, err := s.m2TimingVersion(ctx, name)
		if err != nil {
			return err
		}
		metadata[name] = value
	}
	if err := s.m2VerifyTools(ctx, s.root, s.env, s.m2.Bin); err != nil {
		return err
	}
	for _, tool := range runner.Toolchain() {
		metadata[tool.Name] = tool.Version
	}
	return nil
}

func (s *suite) m2TimingVersion(ctx context.Context, name string) (string, error) {
	program, arg := s.goTool, caseVersion
	if name == m2GitCommand {
		program, arg = s.git, "--version"
	}
	got := s.execute(ctx, command{program: program, args: []string{arg}, dir: s.root, env: s.env})
	if err := success(got); err != nil {
		return "", err
	}
	fields := strings.Fields(got.stdout)
	if len(fields) < 3 || fields[0] != name || fields[1] != caseVersion || !m2TimingVersionValid(name, fields[2]) {
		return "", fmt.Errorf("unreadable timing %s version: %w", filepath.Base(program), ErrAcceptance)
	}
	return fields[2], nil
}

func m2TimingVersionValid(name, value string) bool {
	if name == "go" {
		return version.IsValid(value)
	}
	return regexp.MustCompile(`^\d+\.\d+(\.[a-zA-Z0-9-]+)*$`).MatchString(value)
}

func (s *suite) m2TimingDatabase(ctx context.Context, metadata map[string]string) error {
	tool, _ := runner.LookupTool(m2Scanner)
	path := filepath.Join(s.m2.Bin, executable(tool.Name))
	got := s.execute(ctx, command{program: path, args: tool.Probe, dir: s.root, env: s.env})
	if err := success(got); err != nil {
		return err
	}
	if !tool.Matches(got.stdout) {
		return fmt.Errorf("unreadable timing database probe: %w", ErrAcceptance)
	}
	for line := range strings.SplitSeq(got.stdout, "\n") {
		if source, ok := strings.CutPrefix(strings.TrimSpace(line), "DB: "); ok && m2TimingDatabaseURL(source) {
			metadata["database_source"] = source
		}
		if updated, ok := strings.CutPrefix(strings.TrimSpace(line), "DB updated: "); ok {
			if normalized := m2TimingDatabaseTimestamp(updated); normalized != "" {
				metadata["database_updated"] = normalized
			}
		}
	}
	return nil
}

func m2TimingDatabaseURL(source string) bool {
	address, err := url.Parse(source)
	return err == nil && (address.Scheme == "https" || address.Scheme == "http") &&
		address.Hostname() != "" && address.User == nil && address.RawQuery == "" && address.Fragment == ""
}

func m2TimingDatabaseTimestamp(reported string) string {
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700 -0700",
	}
	for _, layout := range layouts {
		if updated, err := time.Parse(layout, reported); err == nil {
			return updated.Format(time.RFC3339Nano)
		}
	}
	return ""
}
