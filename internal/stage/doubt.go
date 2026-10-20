package stage

import (
	"path"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

// BaseReason starts the reason of a check the comparison with the base
// left unmeasured: the base cannot disprove its findings, which a rung
// leaves to the pipeline.
const BaseReason = "compare with the base: "

// provable holds back the findings of the index the base cannot disprove:
// where it left a file over the read bound, or stopped before it measured,
// a finding may be one it held, so it is neither the commit's own nor
// clean, whatever the index measured. The outcome is then unmeasured, with
// the reason of the base unless the index left it unmeasured already, and
// the files behind the doubt among its unread files, which a rung names and
// leaves to the pipeline (section 5.2).
func (b *baseRun) provable(got *model.Result, base model.Result) {
	if base.Status.Kind != model.StatusUnmeasured {
		return
	}
	var behind []string
	kept := slices.DeleteFunc(slices.Clone(got.Findings), func(f model.Finding) bool {
		unread, doubted := b.doubt(f, base.Unread)
		behind = append(behind, unread...)
		return doubted
	})
	if len(kept) == len(got.Findings) {
		return
	}
	if got.Status.Kind == model.StatusMeasured {
		got.Status = model.Unmeasured(BaseReason + base.Status.Reason)
	}
	for _, p := range behind {
		if !slices.Contains(got.Unread, p) {
			got.Unread = append(got.Unread, p)
		}
	}
	got.Findings = kept
}

// doubt answers whether a finding may be one the base held though its run
// reports none, and the files behind the doubt as the index names them: a
// file the base left unread in the directory of the file the finding sits
// in, that file itself included, because a check reads its directory with
// it (the LogValue of a struct, a test that imports goleak). A base that
// stopped before it measured disproves no finding, and a file it does not
// hold, such as a new one, holds none of its findings.
func (b *baseRun) doubt(f model.Finding, unread []string) ([]string, bool) {
	if len(unread) == 0 {
		return nil, true
	}
	from, held := b.holds[f.Path]
	if !held {
		return nil, false
	}
	var behind []string
	for _, u := range unread {
		if path.Dir(u) == path.Dir(from) {
			behind = append(behind, renamedTo(u, b.moved))
		}
	}
	return behind, len(behind) > 0
}
