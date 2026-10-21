package codex

import (
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// PatchGrammar is the Codex release whose apply_patch grammar the decoder
// reads, the version doctor compares with the installed one.
const PatchGrammar = "0.163.0"

// notBuilt is the cause of every event until the adapter is built: the
// hook fails closed meanwhile.
func notBuilt() error {
	return fmt.Errorf("%w: the codex adapter is not built yet", model.ErrCannotJudge)
}

// Parse decodes an event once: the envelope, the header enrollment reads
// and the decoder of its request.
func Parse([]byte) (hook.Parsed, error) {
	return hook.Parsed{}, notBuilt()
}

// Oversized parses an event over the bound of the hook in bounded memory.
func Oversized(io.Reader) (hook.Parsed, error) {
	return hook.Parsed{}, notBuilt()
}

// Encode answers a result in the shape the host reads: until the adapter
// is built, every result leaves as the cannot-judge answer, exit 2 with a
// stderr line that is never empty, because exit 2 with an empty stderr
// fails open on Codex.
func Encode(_ model.Event, result hook.Result) hook.Reply {
	reason := result.Reason
	if reason == "" {
		reason = notBuilt().Error()
	}
	return hook.Reply{ExitCode: 2, Stderr: reason + "\n"}
}
