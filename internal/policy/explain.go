package policy

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The sources config explain names for a value.
const (
	SourceCore       = "core default"
	SourceEnrollment = "enrollment record"
	SourceComputed   = "computed"
)

// Line is one value of the effective policy with where it came from.
type Line struct {
	Key    string
	Value  string
	Source string
}

// Explain lists every value of the policy in force and its source: the
// core default, the file (named by its origin), the enrollment record or
// a computation, in a fixed order (config explain).
func Explain(l *Loaded) []Line {
	e := explainer{origin: l.Origin.String(), lines: []Line{}}
	e.add("golaw", strconv.Itoa(Schema), SourceCore)
	e.head(l)
	e.floors(l.Policy, l.File)
	e.options(l.Policy)
	e.records(l.File)
	e.add("enrollment.state", l.Policy.Enrollment.State.String(), SourceEnrollment)
	e.add("enrollment.policy_digest", l.Policy.Enrollment.PolicyDigest, SourceEnrollment)
	e.add("enrollment.profile_digest", l.Policy.Enrollment.ProfileDigest, SourceEnrollment)
	e.add("digest", l.Policy.Digest, SourceComputed)
	return e.lines
}

// WriteExplain prints the lines as "key = value (source)", one per line.
func WriteExplain(w io.Writer, lines []Line) error {
	for _, line := range lines {
		if _, err := fmt.Fprintf(w, "%s = %s (%s)\n", line.Key, line.Value, line.Source); err != nil {
			return fmt.Errorf("write the explanation: %w", err)
		}
	}
	return nil
}

// explainer accumulates lines, naming the file origin for set values.
type explainer struct {
	origin string
	lines  []Line
}

func (e *explainer) add(key, value, source string) {
	e.lines = append(e.lines, Line{Key: key, Value: value, Source: source})
}

// set adds a value from the file, or the default from the core.
func (e *explainer) set(key, value string, fromFile bool) {
	if fromFile {
		e.add(key, value, e.origin)
		return
	}
	e.add(key, value, SourceCore)
}

// head explains the pin, the profile, the language, the capabilities and
// the layout.
func (e *explainer) head(l *Loaded) {
	e.set("kit.version", l.Policy.KitVersion, l.File.Kit.Version != "")
	if p := l.File.Profile; p != nil {
		e.add("profile.name", p.Name, e.origin)
		e.add("profile.source", p.Source, e.origin)
		e.add("profile.version", p.Version, e.origin)
		e.add("profile.digest", p.Digest, e.origin)
	} else {
		e.add("profile", "none", SourceCore)
	}
	e.set("language", l.Policy.Language, l.File.Language != nil)
	e.set(keyCapabilities, list(l.Policy.Capabilities), l.File.Capabilities != nil)
	main, decisions := "root", "docs/adr"
	if lay := l.File.Layout; lay != nil {
		if lay.Main != nil {
			main = *lay.Main
		}
		if lay.Decisions != nil {
			decisions = *lay.Decisions
		}
	}
	e.set("layout.main", main, l.File.Layout != nil && l.File.Layout.Main != nil)
	e.set("layout.decisions", decisions, l.File.Layout != nil && l.File.Layout.Decisions != nil)
}

// floors explains the size, term and shell floors.
func (e *explainer) floors(p *model.EffectivePolicy, f File) {
	s := f.Checks.Sizes
	e.set(keyFileLines, strconv.Itoa(p.Sizes.FileLines), s != nil && s.FileLines != nil)
	e.set(keyFunctionLines, strconv.Itoa(p.Sizes.FunctionLines), s != nil && s.FunctionLines != nil)
	t := f.Checks.Terms
	e.set("checks.terms.tools", list(p.Terms.Tools), t != nil && t.Tools != nil)
	e.set(keyAllowPaths, list(p.Terms.AllowPaths), t != nil && t.AllowPaths != nil)
	sh := f.Checks.Shell
	e.set("checks.shell.deny_push", strconv.FormatBool(p.Shell.DenyPush), sh != nil && sh.DenyPush != nil)
	e.set("checks.shell.deny_pr", strconv.FormatBool(p.Shell.DenyPR), sh != nil && sh.DenyPR != nil)
}

// options explains every option the file set, per check id.
func (e *explainer) options(p *model.EffectivePolicy) {
	for _, id := range slices.Sorted(maps.Keys(p.Options)) {
		for _, name := range slices.Sorted(maps.Keys(p.Options[id])) {
			e.add("checks."+id+"."+name, value(p.Options[id][name]), e.origin)
		}
	}
}

// records explains the waivers, the exceptions, the ladder and the
// managed files as the file carries them.
func (e *explainer) records(f File) {
	for i, w := range f.Waivers {
		e.add(fmt.Sprintf("vulnerability_waivers[%d]", i), w.ID+" until "+w.Until+" by "+w.Record, e.origin)
	}
	for i, x := range f.Exceptions {
		e.add(fmt.Sprintf("exceptions[%d]", i), x.Check+" at "+x.Path+" until "+x.Until+" by "+x.Record, e.origin)
	}
	if lad := f.Ladder; lad != nil {
		for _, rung := range []struct {
			key string
			on  *bool
		}{{"pre_commit", lad.PreCommit}, {"commit_msg", lad.CommitMsg}, {"pre_push", lad.PrePush}, {"test_staged", lad.TestStaged}} {
			if rung.on != nil {
				e.add("ladder."+rung.key, strconv.FormatBool(*rung.on), e.origin)
			}
		}
	}
	for i, m := range f.Managed {
		e.add(fmt.Sprintf("managed[%d]", i), m.Path+" "+m.SHA256, e.origin)
	}
}

// list renders a string list for a line.
func list(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	return "[" + strings.Join(items, ", ") + "]"
}

// value renders an option value for a line.
func value(v model.OptionValue) string {
	switch v.Kind {
	case model.OptionInt:
		return strconv.Itoa(v.Int)
	case model.OptionBool:
		return strconv.FormatBool(v.Bool)
	case model.OptionStrings:
		return list(v.Strings)
	default:
		return "unknown"
	}
}
