package model

import "fmt"

// OptionKind is the type of one check option.
type OptionKind int

// The three option kinds a check may declare.
const (
	OptionInt     OptionKind = iota + 1 // an integer floor or limit
	OptionBool                          // a switch
	OptionStrings                       // a list of strings
)

// Valid reports whether the kind is one of the three.
func (k OptionKind) Valid() bool { return k >= OptionInt && k <= OptionStrings }

// Tighten names the direction in which a changed value keeps the policy a
// tightening (section 5.3): a floor only ever moves one way.
type Tighten int

// The tightening directions.
const (
	TightenNone   Tighten = iota + 1 // a switch, free to change
	TightenDown                      // lower is stricter: lengths
	TightenUp                        // higher is stricter: percentages
	TightenShrink                    // fewer entries is stricter: allowlists, exemptions
	TightenGrow                      // more entries is stricter: required sets
)

// Valid reports whether the direction is one of the five.
func (t Tighten) Valid() bool { return t >= TightenNone && t <= TightenGrow }

// OptionSpec declares one option a check reads from the policy.
type OptionSpec struct {
	Name    string      // the key under the check in golaw.yml, lower case
	Kind    OptionKind  // the value type
	Tighten Tighten     // the direction a change may take
	Default OptionValue // the core value, of the same kind
}

// OptionSchema is the ordered list of options a check declares.
type OptionSchema []OptionSpec

// Validate refuses a schema with a repeated name, an invalid kind or a
// default of another kind.
func (s OptionSchema) Validate() error {
	seen := make(map[string]bool, len(s))
	for _, o := range s {
		switch {
		case o.Name == "":
			return fmt.Errorf("%w: option without a name", ErrInvalidSpec)
		case seen[o.Name]:
			return fmt.Errorf("%w: option %q declared twice", ErrInvalidSpec, o.Name)
		case !o.Kind.Valid() || !o.Tighten.Valid():
			return fmt.Errorf("%w: option %q has an invalid kind or direction", ErrInvalidSpec, o.Name)
		case o.Default.Kind != o.Kind:
			return fmt.Errorf("%w: option %q default is not of its kind", ErrInvalidSpec, o.Name)
		}
		seen[o.Name] = true
	}
	return nil
}

// Defaults returns the options with every value at its core default.
func (s OptionSchema) Defaults() Options {
	out := make(Options, len(s))
	for _, o := range s {
		out[o.Name] = o.Default
	}
	return out
}

// OptionValue is one resolved option value; Kind says which field holds it.
type OptionValue struct {
	Kind    OptionKind
	Int     int
	Bool    bool
	Strings []string
}

// IntValue builds an integer option value.
func IntValue(v int) OptionValue { return OptionValue{Kind: OptionInt, Int: v} }

// BoolValue builds a switch option value.
func BoolValue(v bool) OptionValue { return OptionValue{Kind: OptionBool, Bool: v} }

// StringsValue builds a list option value.
func StringsValue(v ...string) OptionValue {
	return OptionValue{Kind: OptionStrings, Strings: v}
}

// Options maps option names to resolved values for one check.
type Options map[string]OptionValue

// Int returns the integer option, false when absent or of another kind.
func (o Options) Int(name string) (int, bool) {
	v, ok := o[name]
	return v.Int, ok && v.Kind == OptionInt
}

// Bool returns the switch option, false when absent or of another kind.
func (o Options) Bool(name string) (bool, bool) {
	v, ok := o[name]
	return v.Bool, ok && v.Kind == OptionBool
}

// Strings returns the list option, false when absent or of another kind.
func (o Options) Strings(name string) ([]string, bool) {
	v, ok := o[name]
	return v.Strings, ok && v.Kind == OptionStrings
}
