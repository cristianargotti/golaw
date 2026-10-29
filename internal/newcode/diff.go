package newcode

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

const (
	devNull    = "/dev/null"
	newPrefix  = "b/"
	goPathspec = "*.go"
	diffTree   = "diff-tree"
	headerMin  = 4
)

// NewLines names, per module-relative path, the lines HEAD adds to the Go
// files since the merge base of base and HEAD, which is what git diff
// --unified=0 <base>...HEAD shows (M6.8 item 4). Renames are off because
// Sonar's blame counts a moved line as new, and --text keeps a Go file an
// attribute marks binary measurable. The plumbing form keeps the person's
// porcelain configuration from changing the shape of the output.
func NewLines(ctx context.Context, root, base string) (map[string][]int, error) {
	fork, err := MergeBase(ctx, root, base)
	if err != nil {
		return nil, err
	}
	out, err := git(ctx, root, diffTree, "-r", "-p", "--unified=0", "--no-renames", "--text",
		"--no-color", "--no-ext-diff", "--src-prefix=a/", "--dst-prefix="+newPrefix,
		fork, headRef, "--", goPathspec)
	if err != nil {
		return nil, err
	}
	if out.Code != 0 {
		return nil, failure(diffTree, out)
	}
	if err := bounded(out); err != nil {
		return nil, err
	}
	return parseDiff(string(out.Stdout))
}

// bounded refuses a diff the runner cut, because a cut diff could only
// name fewer new lines and so raise the ratio.
func bounded(out runner.Output) error {
	if len(out.Stdout) >= runner.MaxOutputBytes {
		return fmt.Errorf("%w: the diff reaches the %d byte bound of the runner, name a nearer --base", ErrDiff, runner.MaxOutputBytes)
	}
	return nil
}

type hunks struct {
	path           string
	removed, added int
	out            map[string][]int
}

func parseDiff(text string) (map[string][]int, error) {
	h := &hunks{out: map[string][]int{}}
	for line := range strings.SplitSeq(text, "\n") {
		if err := h.read(line); err != nil {
			return nil, err
		}
	}
	return h.out, nil
}

// read counts the body of a hunk by its header, which is what keeps a
// content line that starts with +++ or @@ from being read as a header.
func (h *hunks) read(line string) error {
	if h.removed > 0 || h.added > 0 {
		switch {
		case strings.HasPrefix(line, "-"):
			h.removed--
		case strings.HasPrefix(line, "+"):
			h.added--
		}
		return nil
	}
	switch {
	case strings.HasPrefix(line, "+++ "):
		path, err := pathOf(line)
		h.path = path
		return err
	case strings.HasPrefix(line, "@@ "):
		return h.header(line)
	}
	return nil
}

func (h *hunks) header(line string) error {
	fields := strings.Fields(line)
	if len(fields) < headerMin {
		return fmt.Errorf("%w: hunk header %q", ErrDiff, line)
	}
	_, removed, err := span(fields[1], "-")
	if err != nil {
		return err
	}
	start, added, err := span(fields[2], "+")
	if err != nil {
		return err
	}
	h.removed, h.added = removed, added
	if h.path == "" {
		return nil
	}
	for number := start; number < start+added; number++ {
		h.out[h.path] = append(h.out[h.path], number)
	}
	return nil
}

func span(field, sign string) (int, int, error) {
	bad := fmt.Errorf("%w: hunk range %q", ErrDiff, field)
	rest, signed := strings.CutPrefix(field, sign)
	if !signed {
		return 0, 0, bad
	}
	first, second, counted := strings.Cut(rest, ",")
	start, err := strconv.Atoi(first)
	if err != nil || start < 0 {
		return 0, 0, bad
	}
	count := 1
	if counted {
		if count, err = strconv.Atoi(second); err != nil || count < 0 {
			return 0, 0, bad
		}
	}
	return start, count, nil
}

// pathOf reads the new name of a +++ line: git appends a tab after a name
// that holds a space and C-quotes a name with a control character.
func pathOf(line string) (string, error) {
	name := strings.TrimSuffix(strings.TrimPrefix(line, "+++ "), "\t")
	if name == devNull {
		return "", nil
	}
	if strings.HasPrefix(name, `"`) {
		unquoted, err := strconv.Unquote(name)
		if err != nil {
			return "", fmt.Errorf("%w: path %s", ErrDiff, name)
		}
		name = unquoted
	}
	path, prefixed := strings.CutPrefix(name, newPrefix)
	if !prefixed {
		return "", fmt.Errorf("%w: path %s", ErrDiff, name)
	}
	return path, nil
}
