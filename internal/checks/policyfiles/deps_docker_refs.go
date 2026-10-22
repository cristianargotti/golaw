package policyfiles

import (
	"strings"
)

// dockerCopy is the instruction whose --from names a stage or an image.
const dockerCopy = "COPY"

// dockerRef is an image a stage reads beside its base: the --from of COPY or
// the from of a RUN mount, with the instruction that names it and the count
// of the stages declared before it, the only ones it can name.
type dockerRef struct {
	from   string
	digest string
	flag   string
	line   int
	stages int
}

// reference records the images the leading flags of a COPY or RUN name. An
// expansion is recorded as it is, for the image check to state as a limit,
// so a check that reads no reference keeps its answer.
func (d *dockerfile) reference(command, rest string, line int) {
	for text := strings.TrimSpace(rest); strings.HasPrefix(text, "--"); text = strings.TrimSpace(text) {
		word, tail, ok := dockerToken(text)
		if !ok {
			return
		}
		text = tail
		if value, flag, found := dockerFromFlag(command, word); found {
			d.refs = append(d.refs, dockerRef{from: value, digest: dockerDigest(value), flag: flag + value, line: line, stages: len(d.stages)})
		}
	}
}

// dockerFromFlag answers the image a flag word names: --from= of COPY, or
// the from field of a --mount= of RUN.
func dockerFromFlag(command, word string) (string, string, bool) {
	if command == dockerCopy {
		value, found := strings.CutPrefix(word, "--from=")
		return value, "COPY --from=", found
	}
	mount, found := strings.CutPrefix(word, "--mount=")
	for field := range strings.SplitSeq(mount, ",") {
		if value, named := strings.CutPrefix(field, "from="); found && named {
			return value, "RUN --mount from=", true
		}
	}
	return "", "", false
}
