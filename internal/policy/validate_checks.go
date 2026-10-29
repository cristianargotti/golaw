package policy

import (
	"fmt"
	"maps"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

// validate judges the named floor blocks and the per-check options.
func (c Checks) validate(schemas Schemas) error {
	for _, err := range []error{
		c.Sizes.validate(),
		c.Coverage.validate(),
		c.Terms.validate(),
		c.Lint.validate(),
	} {
		if err != nil {
			return err
		}
	}
	for _, id := range slices.Sorted(maps.Keys(c.Options)) {
		if err := validateOptions(id, c.Options[id], schemas); err != nil {
			return err
		}
	}
	return nil
}

// validate keeps each size floor between 1 and its core default.
func (s *Sizes) validate() error {
	if s == nil {
		return nil
	}
	if err := within(keyFileLines, s.FileLines, 1, model.DefaultFileLines); err != nil {
		return err
	}
	return within(keyFunctionLines, s.FunctionLines, 1, model.DefaultFunctionLines)
}

// validate keeps each percentage between the core floor and 100, the
// statement minimum between 0 and the core default, and the exemptions
// clean relative paths.
func (c *Coverage) validate() error {
	if c == nil {
		return nil
	}
	percentages := map[string]*int{"total": c.Total, "package": c.Package, "file": c.File, "new_code": c.NewCode}
	for _, name := range slices.Sorted(maps.Keys(percentages)) {
		if err := within("checks.coverage."+name, percentages[name], DefaultCoverage, percentCap); err != nil {
			return err
		}
	}
	if err := within("checks.coverage.min_statements", c.MinStatements, 0, DefaultMinStatements); err != nil {
		return err
	}
	return cleanPaths("checks.coverage.file_exemptions", c.FileExemptions)
}

// validate keeps the tool names lower case and unique and the allowed
// paths clean, relative and unique.
func (t *Terms) validate() error {
	if t == nil {
		return nil
	}
	for i, tool := range t.Tools {
		if !namePattern.MatchString(tool) {
			return invalid("checks.terms.tools[%d] %q is not a lower case name", i, tool)
		}
		if slices.Contains(t.Tools[:i], tool) {
			return invalid("checks.terms.tools repeats %q", tool)
		}
	}
	return cleanPaths(keyAllowPaths, t.AllowPaths)
}

// validate keeps every exclusion complete.
func (l *Lint) validate() error {
	if l == nil {
		return nil
	}
	for i, e := range l.AllowedExclusions {
		key := fmt.Sprintf("checks.lint.allowed_exclusions[%d]", i)
		if e.Path == "" || len(e.Linters) == 0 {
			return invalid("%s needs a path and at least one linter", key)
		}
		for j, linter := range e.Linters {
			if !namePattern.MatchString(linter) || slices.Contains(e.Linters[:j], linter) {
				return invalid("%s.linters %q is not a unique lower case name", key, linter)
			}
		}
	}
	return nil
}

// validateOptions judges the options of one check against its schema: the
// id must be a check that declares options, every option must be declared,
// every value must be of its kind and none may move from its core default
// against the direction its schema declares, because an option is a floor
// like the named blocks and only tightens the core.
func validateOptions(id string, values map[string]any, schemas Schemas) error {
	schema, known := schemas[id]
	if !model.ValidID(id) || !known {
		return invalid("checks.%s is not a floor block and no check declares options under it", id)
	}
	for _, name := range slices.Sorted(maps.Keys(values)) {
		i := slices.IndexFunc(schema, func(o model.OptionSpec) bool { return o.Name == name })
		if i < 0 {
			return invalid("checks.%s.%s is not an option of the check", id, name)
		}
		v, err := optionValue(schema[i].Kind, values[name])
		if err != nil {
			return invalid("checks.%s.%s: %v", id, name, err)
		}
		if err := tightens("checks."+id+"."+name, schema[i].Tighten, schema[i].Default, v); err != nil {
			return invalid("%v: an option may only tighten its core default", err)
		}
	}
	return nil
}

// optionValue converts a decoded YAML value to the kind an option expects.
func optionValue(kind model.OptionKind, raw any) (model.OptionValue, error) {
	switch v := raw.(type) {
	case int:
		if kind == model.OptionInt {
			return model.IntValue(v), nil
		}
	case bool:
		if kind == model.OptionBool {
			return model.BoolValue(v), nil
		}
	case []any:
		if kind == model.OptionStrings {
			return stringsValue(v)
		}
	}
	return model.OptionValue{}, invalid("value %v is not of the option's kind", raw)
}

// stringsValue converts a decoded list whose every element is a string.
func stringsValue(items []any) (model.OptionValue, error) {
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			return model.OptionValue{}, invalid("list element %v is not a string", item)
		}
		out = append(out, s)
	}
	return model.StringsValue(out...), nil
}

// within keeps an optional integer inside a closed range.
func within(key string, v *int, low, high int) error {
	if v != nil && (*v < low || *v > high) {
		return invalid("%s %d is outside %d to %d", key, *v, low, high)
	}
	return nil
}

// cleanPaths validates a list of clean relative paths without repeats.
func cleanPaths(key string, paths []string) error {
	for i, p := range paths {
		if err := relPath(fmt.Sprintf("%s[%d]", key, i), p); err != nil {
			return err
		}
		if slices.Contains(paths[:i], p) {
			return invalid("%s repeats %q", key, p)
		}
	}
	return nil
}
