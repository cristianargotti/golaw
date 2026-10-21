package policyfiles

import (
	"context"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const squawkFile = ".squawk.toml"

var squawkVersion = regexp.MustCompile(`^\d+(?:\.\d+)*$`)

type squawkConfig struct{}

// SquawkConfig keeps migration safety requirements in the configuration the tool reads.
func SquawkConfig() model.Check { return squawkConfig{} }

func (squawkConfig) Spec() model.Spec { return spec("squawk-config", ruleData, "db") }

// Run decodes the TOML whole and reads only pg_version,
// assume_in_transaction, excluded_rules and excluded_paths: another type for
// one of them is outside, at line 0 because the decoder gives no key line,
// and every other key is skipped, since squawk alone gives it meaning.
func (squawkConfig) Run(_ context.Context, in model.Input) (model.Result, error) {
	data, found, err := read(in, squawkFile)
	if err != nil {
		return answer(squawkFile, err), nil
	}
	result := model.Result{Status: model.Measured(), Scope: squawkFile}
	if !found {
		message := "add the missing " + squawkFile + ` with pg_version = "17.0", assume_in_transaction = true` +
			` and excluded_rules = ["prefer-identity"]; init writes it under the db capability`
		result.Findings = []model.Finding{squawkFinding(message)}
		return result, nil
	}
	var values map[string]any
	if parseErr := decodeTOML(squawkFile, data, &values); parseErr != nil {
		return answer(squawkFile, parseErr), nil
	}
	repairs, err := squawkRepairs(values)
	if err != nil {
		return answer(squawkFile, err), nil
	}
	excluded, err := squawkStrings(values, "excluded_paths")
	if err != nil {
		return answer(squawkFile, err), nil
	}
	if len(repairs) != 0 {
		result.Findings = append(result.Findings, squawkFinding("in "+squawkFile+" set "+strings.Join(repairs, " and ")))
	}
	result.Findings = append(result.Findings, squawkExclusions(in.Snapshot, excluded)...)
	return result, nil
}

func squawkRepairs(values map[string]any) ([]string, error) {
	version, err := squawkValue[string](values, "pg_version")
	if err != nil {
		return nil, err
	}
	transaction, err := squawkValue[bool](values, "assume_in_transaction")
	if err != nil {
		return nil, err
	}
	rules, err := squawkStrings(values, "excluded_rules")
	if err != nil {
		return nil, err
	}
	var repairs []string
	if !squawkVersion.MatchString(version) {
		repairs = append(repairs, `pg_version = "17.0" or another numeric PostgreSQL version`)
	}
	if !transaction {
		repairs = append(repairs, "assume_in_transaction = true")
	}
	if slices.ContainsFunc(rules, func(rule string) bool { return rule != "prefer-identity" }) {
		repairs = append(repairs, `excluded_rules = [] or ["prefer-identity"]`)
	}
	return repairs, nil
}

func squawkValue[T any](values map[string]any, key string) (T, error) {
	var zero T
	value, found := values[key]
	if !found {
		return zero, nil
	}
	typed, ok := value.(T)
	if !ok {
		return zero, outside(squawkFile, 0, "unsupported value type for "+key)
	}
	return typed, nil
}

func squawkStrings(values map[string]any, key string) ([]string, error) {
	items, err := squawkValue[[]any](values, key)
	if err != nil {
		return nil, err
	}
	list := make([]string, 0, len(items))
	for _, item := range items {
		value, ok := item.(string)
		if !ok {
			return nil, outside(squawkFile, 0, "unsupported "+key+" entry; write a list of strings")
		}
		list = append(list, value)
	}
	return list, nil
}

// squawkExclusions judges the migrations the MIGRATIONS default of golaw.mk
// selects and the sample migration, because an entry elsewhere excludes
// nothing the pipeline lints.
func squawkExclusions(snapshot model.Snapshot, entries []string) []model.Finding {
	files := model.Select(snapshot, func(file string) bool {
		matched, err := path.Match("db/migrations/*.sql", file)
		return err == nil && matched
	})
	files = append(files, model.File{Path: "db/migrations/0001_example.sql"})
	var findings []model.Finding
	for _, entry := range entries {
		if slices.ContainsFunc(files, func(file model.File) bool { return squawkExcludes(entry, file.Path) }) {
			message := "remove " + strconv.Quote(entry) + " from excluded_paths in " + squawkFile +
				" to keep the migration under squawk or record its exception in golaw.yml"
			findings = append(findings, squawkFinding(message))
		}
	}
	return findings
}

func squawkExcludes(entry, file string) bool {
	if strings.HasSuffix(entry, "/") || strings.HasSuffix(entry, "/**") {
		directory := strings.TrimSuffix(strings.TrimSuffix(entry, "**"), "/")
		if strings.HasPrefix(file, directory+"/") {
			return true
		}
	}
	matched, err := path.Match(entry, file)
	if err == nil && matched {
		return true
	}
	rest, recursive := strings.CutPrefix(entry, "**/")
	if !recursive {
		return false
	}
	for {
		if squawkExcludes(rest, file) {
			return true
		}
		_, tail, found := strings.Cut(file, "/")
		if !found {
			return false
		}
		file = tail
	}
}

func squawkFinding(message string) model.Finding {
	return finding(squawkFile, 0, "squawk-policy", message)
}
