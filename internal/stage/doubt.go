package stage

import (
	"path"
	"slices"
	"strings"

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
	holdBack(got, base, func(f model.Finding) ([]string, bool) { return b.doubt(f, base.Unread) })
}

// besideDoubt holds back a package finding of the index whose package the
// run of the files beside the targets left a Go file unread in: the rule
// judges the package whole, so the file it could not read may start the
// goroutines the base held, and the finding is no proof of the commit.
func (b *baseRun) besideDoubt(got *model.Result, i int) {
	if b.beside == nil || b.beside.Outcomes[i].Result.Status.Kind != model.StatusUnmeasured {
		return
	}
	base := b.beside.Outcomes[i].Result
	holdBack(got, base, func(f model.Finding) ([]string, bool) {
		var behind []string
		for _, u := range base.Unread {
			if f.Check == packageRule && path.Dir(u) == path.Dir(f.Path) && strings.HasSuffix(u, goSuffix) {
				behind = append(behind, renamedTo(u, b.moved))
			}
		}
		return behind, len(behind) > 0
	})
}

// holdBack removes each finding doubt answers doubted, and leaves the
// outcome unmeasured with the reason of the base, unless the index left it
// unmeasured already, and the files behind each doubt among its unread
// files.
func holdBack(got *model.Result, base model.Result, doubt func(model.Finding) ([]string, bool)) {
	var unread []string
	kept := slices.DeleteFunc(slices.Clone(got.Findings), func(f model.Finding) bool {
		files, doubted := doubt(f)
		unread = append(unread, files...)
		return doubted
	})
	if len(kept) == len(got.Findings) {
		return
	}
	if got.Status.Kind == model.StatusMeasured {
		got.Status = model.Unmeasured(BaseReason + base.Status.Reason)
	}
	for _, p := range unread {
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
