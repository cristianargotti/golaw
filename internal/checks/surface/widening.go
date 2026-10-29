package surface

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

const (
	wideningCode   = "analyzer-widening"
	wideningTail   = " from the pipeline's gate; fix the findings, or ask the person to change the scope."
	keySources     = "sonar.sources"
	issueKeyPrefix = "sonar.issue.ignore."
)

// hidingKeys are the list keys of the properties subset an entry added to
// hides code from the gate, in report order.
func hidingKeys() []string {
	return []string{"sonar.exclusions", "sonar.coverage.exclusions", "sonar.cpd.exclusions", "sonar.test.inclusions"}
}

// analyzerWidening judges an agent's write of the root properties file
// against its base. It is no file check, because it compares two versions
// of one file and only in a write event.
type analyzerWidening struct{ spec model.Spec }

// AnalyzerWidening denies an agent's write of sonar-project.properties
// that adds an exclusion or an issue ignore key or narrows the sources,
// compared with the base file (M6.7 check 6); a person edits the file as
// before, because no rung or scan runs it.
func AnalyzerWidening() model.Check {
	s := spec("surface.analyzer-widening")
	s.Scope = model.ScopeEvent
	return analyzerWidening{spec: s}
}

// Spec returns the check specification.
func (w analyzerWidening) Spec() model.Spec { return w.spec }

// Run judges the candidate of the root properties file. Smallest behavior:
// a post event changes nothing the pre event did not already judge, so it
// is not applicable there.
func (w analyzerWidening) Run(_ context.Context, in model.Input) (model.Result, error) {
	if in.Mode != model.ModeHook || !writeEvent(in) {
		return model.Result{Status: model.NotApplicable("an agent's write of " + detect.PropertiesFile + " only")}, nil
	}
	res := model.Result{Status: model.Measured(), Scope: "0 files"}
	for _, c := range in.Event.Candidates {
		if c.Path != detect.PropertiesFile || c.Op == model.CandidateDelete || !c.Reconstructed {
			continue
		}
		candidate, err := in.Snapshot.Read(c.Path)
		if errors.Is(err, model.ErrTooLarge) {
			res.Unread = append(res.Unread, c.Path)
			continue
		}
		if err != nil {
			return model.Result{Status: model.Unmeasured(err.Error())}, nil
		}
		base, ok := baseOf(in, c)
		if !ok {
			continue
		}
		res.Scope = "1 files"
		res.Findings = append(res.Findings, widened(base, candidate)...)
	}
	return res, nil
}

// baseOf answers the base bytes of the candidate. Smallest behavior: an
// edit whose base golaw cannot read is never denied, while a created file
// has an empty base.
func baseOf(in model.Input, c model.Candidate) ([]byte, bool) {
	based, ok := in.Snapshot.(model.Based)
	if !ok {
		return nil, c.Op == model.CandidateCreate
	}
	src, err := based.Base().Read(basePath(in.Event, c.Path))
	if errors.Is(err, model.ErrNotInSnapshot) {
		return nil, true
	}
	return src, err == nil
}

// widened answers the findings of the candidate against the base, all at
// line 0 because the file as a whole sets the scope.
func widened(baseSrc, candidateSrc []byte) []model.Finding {
	base, candidate := detect.ParseProperties(baseSrc), detect.ParseProperties(candidateSrc)
	skip := unmeasuredKeys(base, candidate)
	var messages []string
	for _, key := range hidingKeys() {
		if !skip[key] {
			for _, entry := range added(base.Values[key], candidate.Values[key]) {
				messages = append(messages, fmt.Sprintf("adds %s to %s: an exclusion hides code%s", entry, key, wideningTail))
			}
		}
	}
	if !skip[keySources] {
		if list, ok := narrowed(base.Values, candidate.Values); ok {
			messages = append(messages, "narrows "+keySources+" to "+list+": a narrower scope hides code"+wideningTail)
		}
	}
	for _, key := range added(issueKeys(baseSrc), issueKeys(candidateSrc)) {
		messages = append(messages, "adds "+key+", which hides findings"+wideningTail)
	}
	out := make([]model.Finding, 0, len(messages))
	for _, m := range messages {
		out = append(out, model.Finding{Path: detect.PropertiesFile, Code: wideningCode, Message: m, Severity: model.SeverityDeny})
	}
	return out
}

// unmeasuredKeys are the keys either side names Unmeasured, which are not
// compared because their value is not known.
func unmeasuredKeys(sides ...detect.Properties) map[string]bool {
	out := map[string]bool{}
	for _, p := range sides {
		for _, entry := range p.Unmeasured {
			key, _, _ := strings.Cut(entry, ": ")
			out[key] = true
		}
	}
	return out
}

// added answers the candidate entries the base lacks, each once, in the
// candidate's order.
func added(base, candidate []string) []string {
	var out []string
	for _, entry := range candidate {
		if !slices.Contains(base, entry) && !slices.Contains(out, entry) {
			out = append(out, entry)
		}
	}
	return out
}

// narrowed reports a candidate that names sources narrower than the base:
// a list other than the whole tree where the base names none, or a list
// that drops an entry of the base. An empty or absent candidate list
// leaves the default, the whole tree, so it never narrows.
func narrowed(base, candidate map[string][]string) (string, bool) {
	list := candidate[keySources]
	if len(list) == 0 {
		return "", false
	}
	was, named := base[keySources]
	if !named {
		return strings.Join(list, ","), !slices.Equal(list, []string{"."})
	}
	for _, entry := range was {
		if !slices.Contains(list, entry) {
			return strings.Join(list, ","), true
		}
	}
	return "", false
}

// issueKeys answers the issue ignore keys of the file, read from its
// logical lines up to the first separator, because ParseProperties keeps
// only the keys of the scope.
func issueKeys(data []byte) []string {
	var out []string
	joined, continuing := "", false
	for raw := range strings.SplitSeq(string(data), "\n") {
		line := strings.TrimLeft(strings.TrimSuffix(raw, "\r"), " \t\f")
		if !continuing && (line == "" || line[0] == '#' || line[0] == '!') {
			continue
		}
		trailing := len(line) - len(strings.TrimRight(line, `\`))
		if continuing = trailing%2 == 1; continuing {
			joined += line[:len(line)-1]
			continue
		}
		joined += line
		key := joined
		if end := strings.IndexAny(joined, "=: \t\f"); end >= 0 {
			key = joined[:end]
		}
		if strings.HasPrefix(key, issueKeyPrefix) && !slices.Contains(out, key) {
			out = append(out, key)
		}
		joined = ""
	}
	return out
}
