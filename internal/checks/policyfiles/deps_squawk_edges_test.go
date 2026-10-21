package policyfiles_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestSquawkConfigNumericVersions(t *testing.T) {
	t.Parallel()
	near := depsProof(t, "policy.squawk-config", "near", "db")
	for _, row := range []struct {
		version string
		valid   bool
	}{
		{"9.6", true},
		{"18", true},
		{"17.0.1", true},
		{"", false},
		{"17beta", false},
		{"17.", false},
		{".17", false},
		{"17..0", false},
		{" 17.0", false},
		{"-17", false},
	} {
		body := "pg_version = " + strconv.Quote(row.version) + "\nassume_in_transaction = true\n"
		var want []string
		if !row.valid {
			want = []string{"squawk-policy"}
		}
		assertDeps(t, policyfiles.SquawkConfig(), []depsCase{{
			name: row.version, input: depsEdit(near, ".squawk.toml", body), want: want,
		}})
	}
}

func TestSquawkConfigRelevantTypes(t *testing.T) {
	t.Parallel()
	near := depsProof(t, "policy.squawk-config", "near", "db")
	for _, row := range []struct{ key, value string }{
		{"pg_version", "17"},
		{"pg_version", "[]"},
		{"assume_in_transaction", `"true"`},
		{"assume_in_transaction", "{}"},
		{"excluded_rules", `"prefer-identity"`},
		{"excluded_rules", "[1]"},
		{"excluded_paths", `"vendor/*"`},
		{"excluded_paths", "[false]"},
		{"excluded_paths", `['vendor/*', 1]`},
		{"excluded_paths", "{}"},
	} {
		assertDeps(t, policyfiles.SquawkConfig(), []depsCase{{
			name: row.key + row.value, input: depsEdit(near, ".squawk.toml", row.key+" = "+row.value+"\n"),
			status: model.StatusUnmeasured, reason: row.key, outside: true,
		}})
	}
}

func TestSquawkConfigMigrationExclusions(t *testing.T) {
	t.Parallel()
	near := depsProof(t, "policy.squawk-config", "near", "db")
	near = depsEdit(near, "db/migrations/0002_custom.sql", "CREATE TABLE custom (id bigint);\n")
	near = depsEdit(near, "db/migrations/nested/ignored.sql", "CREATE TABLE ignored (id bigint);\n")
	for _, row := range []struct {
		entry string
		red   bool
	}{
		{"db/migrations/", true},
		{"db/migrations/**", true},
		{"db/", true},
		{"db/**", true},
		{"**/migrations/*.sql", true},
		{"**/0001_example.sql", true},
		{"**/db/migrations/*.sql", true},
		{"**/migrations/**", true},
		{"**/**/0001_example.sql", true},
		{"**/**/**/0001_example.sql", true},
		{"**/**/**/**/0001_example.sql", true},
		{"**/db/**", true},
		{"**/unrelated/*.sql", false},
		{"db/migrations/0002_custom.sql", true},
		{"**/0002_custom.sql", true},
		{"db/migrations/nested/ignored.sql", false},
		{"db/migrations/nested/", false},
		{"db/migrations-old/**", false},
		{"vendor/**", false},
		{"[", false},
		{"db/migrations/[", false},
		{"db/migrations/absent.sql", false},
		{"", false},
	} {
		body := near.files[".squawk.toml"] + "excluded_paths = [" + strconv.Quote(row.entry) + "]\n"
		var want []string
		if row.red {
			want = []string{"squawk-policy"}
		}
		assertDeps(t, policyfiles.SquawkConfig(), []depsCase{{
			name: row.entry, input: depsEdit(near, ".squawk.toml", body), want: want,
		}})
	}
}

func TestSquawkConfigFullTOMLAndMissingValues(t *testing.T) {
	t.Parallel()
	near := depsProof(t, "policy.squawk-config", "near", "db")
	body := near.files[".squawk.toml"]
	assertDeps(t, policyfiles.SquawkConfig(), []depsCase{
		{name: "transaction missing", input: depsEdit(near, ".squawk.toml", `pg_version = "17"`), want: []string{"squawk-policy"}},
		{name: "empty document", input: depsEdit(near, ".squawk.toml", "# empty\n"), want: []string{"squawk-policy"}},
		{name: "multiline arrays and unrelated TOML", input: depsEdit(near, ".squawk.toml", body+`
excluded_paths = [
  'vendor/*',
]
[unrelated]
date = 2026-10-20
literal = '''
excluded_paths = ['db/migrations/*']
'''
[[unrelated.rows]]
value = { nested = [1, true, 'text'] }
`)},
	})
}

func TestSquawkConfigRepairMessages(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		words      []string
	}{
		{"file missing", "", []string{
			"add the missing .squawk.toml", `pg_version = "17.0"`, "assume_in_transaction = true",
			`excluded_rules = ["prefer-identity"]`, "init writes it under the db capability",
		}},
		{"empty document", "# empty\n", []string{"in .squawk.toml set", "pg_version", "assume_in_transaction = true"}},
		{"excluded rules", "excluded_rules = ['other']\n", []string{
			"in .squawk.toml set", "pg_version", "assume_in_transaction", "excluded_rules",
		}},
		{
			"excluded paths",
			"pg_version = '17'\nassume_in_transaction = true\nexcluded_paths = ['db/migrations/**']\n",
			[]string{"excluded_paths", "db/migrations/**", "remove", "golaw.yml"},
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			near := depsProof(t, "policy.squawk-config", "near", "db")
			got := measure(t, policyfiles.SquawkConfig(), depsEdit(near, ".squawk.toml", row.body))
			if len(got.Findings) != 1 {
				t.Fatalf("findings = %+v, want one repair", got.Findings)
			}
			item := got.Findings[0]
			if item.Path != ".squawk.toml" || item.Line != 0 || !strings.Contains(item.Message, item.Path) {
				t.Fatalf("location or message = %+v", item)
			}
			for _, word := range row.words {
				if !strings.Contains(item.Message, word) {
					t.Errorf("message %q does not name %q", item.Message, word)
				}
			}
		})
	}
}
