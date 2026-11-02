package main

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// The m7-regen rows of M7.13 row 3: the summary line of the port edit, the
// hole it appends to the owned half of the SQL adapter, the contracts the
// budget rows edit and the notice of a regeneration left to golaw gen.
const (
	m7RegenCategory  = "m7-regen"
	m7RegenSummary   = "golaw regenerated from internal/application/growth/ports.go (ports Ledger, Clock)"
	m7PurgeHole      = "plans.purge-plans"
	m7QueriesFile    = "db/queries/plans.sql"
	m7NotRegenerated = "not regenerated in the hook: run golaw gen"
	m7OpenAPIFile    = "api/openapi.yaml"
)

// The lines the edits read and write: the method the port gains after the
// one it has, the query sqlc turns into a method, and the summary of the
// GetPlan operation, whose change touches a comment and no hole.
const (
	m7ArchiveLine  = "\tArchivePlan(ctx context.Context, account AccountID) error\n"
	m7PurgeMethod  = "\t// PurgePlans removes the archived plans of one account.\n\tPurgePlans(ctx context.Context, account AccountID) error\n"
	m7PurgeQuery   = "-- name: PurgePlans :execrows\nDELETE FROM plans WHERE account = $1 AND archived;\n"
	m7OldSummary   = "summary: Read the plan of an account"
	m7NewSummary   = "summary: Read the plan of one account"
	m7SwitchOff    = "build: switch the hook regeneration off"
	m7RegenPrefix  = "golaw regenerated"
	m7HoleMarker   = "golaw:hole "
	m7HeredocStart = "cat > " + m7PortsPath + " <<'EOF'\n"
)

// m7PortsWithPurge adds PurgePlans to the Ledger port after ArchivePlan.
func m7PortsWithPurge(ports []byte) ([]byte, error) {
	if n := strings.Count(string(ports), m7ArchiveLine); n != 1 {
		return nil, fmt.Errorf("the ports contract holds %d lines %q, want one: %w", n, m7ArchiveLine, ErrAcceptance)
	}
	return []byte(strings.Replace(string(ports), m7ArchiveLine, m7ArchiveLine+m7PurgeMethod, 1)), nil
}

// m7QueriesWithPurge appends the PurgePlans query one blank line after the
// last query.
func m7QueriesWithPurge(queries []byte) []byte {
	return []byte(strings.TrimRight(string(queries), "\n") + "\n\n" + m7PurgeQuery)
}

// m7OpenAPIWithSummary rewords the summary of the GetPlan operation.
func m7OpenAPIWithSummary(doc []byte) ([]byte, error) {
	if n := strings.Count(string(doc), m7OldSummary); n != 1 {
		return nil, fmt.Errorf("the document holds %d lines %q, want one: %w", n, m7OldSummary, ErrAcceptance)
	}
	return []byte(strings.Replace(string(doc), m7OldSummary, m7NewSummary, 1)), nil
}

// m7JudgeRegenContext wants the summary of the port edit and the hole it
// appended to the owned half of the SQL adapter.
func m7JudgeRegenContext(text string) error {
	if !strings.Contains(text, m7RegenSummary) {
		return fmt.Errorf("the agent context %q lacks %q: %w", text, m7RegenSummary, ErrAcceptance)
	}
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "hole open "+m7OwnedPlans+":") && strings.Contains(line, " "+m7PurgeHole+" ") {
			return nil
		}
	}
	return fmt.Errorf("the agent context %q lacks the hole %s of %s: %w", text, m7PurgeHole, m7OwnedPlans, ErrAcceptance)
}

// m7JudgeRegenFiles reads the checkout: the stub appended to the owned
// file (M7.4.2 rule 5) and the method in the regenerated fake.
func m7JudgeRegenFiles(repo string) error {
	for path, want := range map[string]string{m7OwnedPlans: m7HoleMarker + m7PurgeHole, m7FakePath: "PurgePlans"} {
		body, err := readFile(filepath.Join(repo, filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		if !strings.Contains(string(body), want) {
			return fmt.Errorf("%s lacks %q after the regeneration: %w", path, want, ErrAcceptance)
		}
	}
	return nil
}

// m7WithoutRegenerate switches the regeneration off on the ladder line,
// which stays outside the policy digest (A43), so the enrollment holds.
func m7WithoutRegenerate(policy []byte) ([]byte, error) {
	ladder := regexp.MustCompile(`(?m)^ladder: \{(.*) \}$`)
	match := ladder.FindSubmatchIndex(policy)
	if match == nil {
		return nil, fmt.Errorf("the policy holds no ladder line: %w", ErrAcceptance)
	}
	inner := string(policy[match[2]:match[3]])
	if strings.Contains(inner, "hook_regenerate: true") {
		inner = strings.Replace(inner, "hook_regenerate: true", "hook_regenerate: false", 1)
	} else if !strings.Contains(inner, "hook_regenerate:") {
		inner += ", hook_regenerate: false"
	}
	return append(append(append([]byte{}, policy[:match[2]]...), inner...), policy[match[3]:]...), nil
}

// m7Edited is a denial repository with the Ledger port edited in the
// checkout, as the host writes it before its post event.
func (s *m7Suite) m7Edited(ctx context.Context, name string) (string, []byte, []byte, error) {
	return s.m7EditContract(ctx, name, m7PortsPath, m7PortsWithPurge)
}

// m7RegenCases are the m7-regen rows in report order: the write shapes of
// the three agents, the switch, then the budgets.
func m7RegenCases() []m7Case {
	return []m7Case{
		{name: "claude-write", run: m7RegenRow(func(s *m7Suite, ctx context.Context, repo string, _, after []byte) (string, error) {
			return s.m7ClaudePost(ctx, repo, fixture{name: "m7-regen-write", event: eventPost, tool: toolWrite, input: map[string]any{
				keyFilePath: filepath.Join(repo, filepath.FromSlash(m7PortsPath)), m5KeyContent: string(after),
			}})
		})},
		{name: "claude-edit", run: m7RegenRow(func(s *m7Suite, ctx context.Context, repo string, _, _ []byte) (string, error) {
			return s.m7ClaudePost(ctx, repo, fixture{name: "m7-regen-edit", event: eventPost, tool: toolEdit, input: map[string]any{
				keyFilePath: filepath.Join(repo, filepath.FromSlash(m7PortsPath)), m7OldString: m7ArchiveLine, m7NewString: m7ArchiveLine + m7PurgeMethod, m7ReplaceAll: false,
			}})
		})},
		{name: "codex-apply-patch", run: m7RegenRow((*m7Suite).m7CodexPost)},
		{name: "claude-bash-heredoc", run: m7RegenRow((*m7Suite).m7Heredoc)},
		{name: "cursor-post", run: m7RegenRow((*m7Suite).m7CursorPost)},
		{name: "regenerate-off", run: (*m7Suite).m7RegenerateOff},
		{name: "port-p95", gen: true, run: (*m7Suite).m7PortP95},
		{name: "query-p95", gen: true, run: func(s *m7Suite, ctx context.Context) error {
			return s.m7ToolP95(ctx, "query-p95", m7QueriesFile, func(b []byte) ([]byte, error) { return m7QueriesWithPurge(b), nil })
		}},
		{name: "openapi-p95", gen: true, run: func(s *m7Suite, ctx context.Context) error {
			return s.m7ToolP95(ctx, "openapi-p95", m7OpenAPIFile, m7OpenAPIWithSummary)
		}},
		{name: "slow-generator-killed", run: (*m7Suite).m7SlowGenerator},
	}
}

// m7RegenRow edits the port in a repository of its own, runs one write
// shape and judges the text the agent reads and the files it left.
func m7RegenRow(shape func(s *m7Suite, ctx context.Context, repo string, before, after []byte) (string, error)) func(*m7Suite, context.Context) error {
	return func(s *m7Suite, ctx context.Context) error {
		repo, before, after, err := s.m7Edited(ctx, "regen")
		if err != nil {
			return err
		}
		text, err := shape(s, ctx, repo, before, after)
		if err != nil {
			return err
		}
		if err := m7JudgeRegenContext(text); err != nil {
			return err
		}
		return m7JudgeRegenFiles(repo)
	}
}
