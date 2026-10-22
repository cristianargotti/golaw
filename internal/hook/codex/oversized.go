package codex

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// Oversized parses an event whose input is over the bound of the hook, in
// bounded memory: the whole input is checked as the standard decoder
// checks it, then the copy of its envelope is parsed as any event. A
// command kept whole is judged as one within the bound. A longer command
// is, for apply_patch, the patch its skeleton names, validated whole and
// exposed as the targets of enrollment; its decoder answers the files of
// a patch before it runs by name, with the gap of the input, and the files
// it wrote after; a Bash event gets the detection of its post event, and
// before it runs cannot be judged.
func Oversized(input io.Reader) (hook.Parsed, error) {
	o, err := shrinkEnvelope(input)
	if err != nil {
		return hook.Parsed{}, err
	}
	if o.input == nil || !o.input.elided {
		return Parse(o.document())
	}
	event, in, err := parse(o.document())
	if err != nil {
		return hook.Parsed{}, err
	}
	if event.Tool != patchTool {
		return hook.Parsed{Event: event, Decode: func(base hook.Base, _ model.Snapshot) (hook.Request, error) {
			return unheldCommand(event, in, base)
		}}, nil
	}
	text, err := o.input.patch.close()
	if err != nil {
		return hook.Parsed{}, err
	}
	doc, err := parsePatch(text)
	if err != nil {
		return hook.Parsed{}, err
	}
	var targets []string
	event.Target, targets = patchTargets(event.Cwd, text)
	return hook.Parsed{Event: event, Targets: targets, Decode: func(base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
		if event.Shape == model.ShapePost {
			return postRequest(event, text, in.response, base)
		}
		return unheldPatch(event, doc, base, snapshot)
	}}, nil
}

// unheldCommand is the request of an event whose command is over the bound
// and is no patch: a post Bash event is the detection of what the command
// changed, which needs no command, while a Bash command before it runs
// cannot be judged, because the shell family reads the whole command.
func unheldCommand(event model.Event, in toolInput, base hook.Base) (hook.Request, error) {
	switch {
	case event.Tool != bashTool:
		return hook.Request{}, unsupportedTool(event.Tool)
	case event.Shape == model.ShapePost:
		return postRequest(event, "", in.response, base)
	default:
		return hook.Request{}, fmt.Errorf("%w: %w: the command is over the 8 MiB the hook reads: run it as smaller commands", model.ErrCannotJudge, model.ErrTooLarge)
	}
}

// unheldPatch is the request of a patch over the bound before it runs:
// each file named by its path with the gap of the input in place of its
// bytes, so the core keeps golaw.yml protected, judges the names and names
// what it did not measure; a deletion has no bytes and needs no gap.
func unheldPatch(event model.Event, doc patchDoc, base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
	event.Shape = model.ShapeWrite
	switch {
	case len(doc.Hunks) == 0:
		return hook.Request{Event: event, Observe: true}, nil
	case doc.Environment != "":
		return hook.Request{}, environmentRefusal(doc.Environment)
	}
	request := hook.Request{Event: event}
	for _, hunk := range doc.Hunks {
		candidate, err := unheldCandidate(hunk, event.Cwd, base, snapshot)
		if err != nil {
			return hook.Request{}, err
		}
		request.Event.Candidates = append(request.Event.Candidates, candidate)
		if candidate.Op != model.CandidateDelete {
			request.Gaps = append(request.Gaps, hook.OverInputBound(candidate.Path))
		}
	}
	return request, nil
}

// unheldCandidate is the candidate of one hunk by its paths alone, its
// source required present as the patch route requires it.
func unheldCandidate(hunk Hunk, cwd string, base hook.Base, snapshot model.Snapshot) (model.Candidate, error) {
	path, err := patchPath(base, cwd, hunk.Path)
	if err != nil {
		return model.Candidate{}, err
	}
	switch hunk.Op {
	case model.CandidateCreate, model.CandidateReplace:
		if snapshot.Class(path) != model.ClassUnknown {
			return model.Candidate{Path: path, Op: model.CandidateReplace}, nil
		}
		return model.Candidate{Path: path, Op: model.CandidateCreate}, nil
	case model.CandidateDelete, model.CandidateEdit, model.CandidateMove:
		if snapshot.Class(path) == model.ClassUnknown {
			return model.Candidate{}, absentSource(path)
		}
		return heldSource(hunk, path, cwd, base)
	default:
		return model.Candidate{}, fmt.Errorf("%w: unsupported patch operation %s", model.ErrCannotJudge, hunk.Op)
	}
}

// heldSource is the candidate of a hunk whose source the snapshot holds:
// a deletion, which is whole, a move to its destination, or an edit.
func heldSource(hunk Hunk, path, cwd string, base hook.Base) (model.Candidate, error) {
	if hunk.Op == model.CandidateDelete {
		return model.Candidate{Path: path, Op: model.CandidateDelete, Reconstructed: true}, nil
	}
	if hunk.Op != model.CandidateMove {
		return model.Candidate{Path: path, Op: model.CandidateEdit}, nil
	}
	to, err := patchPath(base, cwd, hunk.To)
	if err != nil {
		return model.Candidate{}, err
	}
	return model.Candidate{Path: to, From: path, Op: model.CandidateMove}, nil
}

// headerLike tells a line that may be a marker or a hunk header in some
// hunk: one whose text after its leading white space starts as every
// marker of the grammar starts.
func headerLike(line []byte) bool {
	text := bytes.TrimLeftFunc(line, unicode.IsSpace)
	return bytes.HasPrefix(text, []byte(headerPrefix)) || bytes.HasPrefix(text, []byte(chunkMarker))
}

// firstRune is the first character of a line, empty for an empty line.
func firstRune(line []byte) []byte {
	_, n := utf8.DecodeRune(line)
	return line[:n]
}

// lastRunes is the end of a line from the start of a character, at least
// n bytes long when the line is.
func lastRunes(line []byte, n int) []byte {
	start := max(0, len(line)-n)
	for start > 0 && !utf8.RuneStart(line[start]) {
		start--
	}
	return line[start:]
}

// validText is a line as the standard decoder hands it over: each byte
// that is not valid UTF-8 replaced by the replacement character.
func validText(line []byte) []byte {
	if utf8.Valid(line) {
		return bytes.Clone(line)
	}
	out := make([]byte, 0, len(line)+utf8.UTFMax)
	for len(line) > 0 {
		r, n := utf8.DecodeRune(line)
		if r == utf8.RuneError && n == 1 {
			out = utf8.AppendRune(out, r)
		} else {
			out = append(out, line[:n]...)
		}
		line = line[n:]
	}
	return out
}

// emptyOf is an empty value of the type a value too long to keep starts:
// an object, an array, or a number, since a literal is never that long.
func emptyOf(first byte) []byte {
	switch first {
	case '{':
		return []byte("{}")
	case '[':
		return []byte("[]")
	default:
		return []byte("0")
	}
}

// appendMember appends a member with its comma.
func appendMember(doc []byte, field string, raw []byte) []byte {
	doc = append(doc, '"')
	doc = append(doc, field...)
	doc = append(doc, '"', ':')
	doc = append(doc, raw...)
	return append(doc, ',')
}

// malformed is the refusal of an input that breaks the JSON grammar.
func malformed(where string) error {
	return fmt.Errorf("%w: an input over the bound is not valid JSON at %s", model.ErrCannotJudge, where)
}

// readFailure is the refusal of an input that ends inside its document or
// cannot be read.
func readFailure(err error) error {
	if errors.Is(err, io.EOF) {
		return malformed("its end, inside the document")
	}
	return fmt.Errorf("%w: read input: %w", model.ErrCannotJudge, err)
}
