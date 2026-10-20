package commits

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The modes git records for a symbolic link and a submodule, entries that
// bring no line.
const (
	linkMode    = "120000"
	gitlinkMode = "160000"
)

// newLines counts the lines the changes bring. A file over the read bound
// is answered aside as unread instead of refused, because its lines cannot
// be counted with certainty.
func newLines(s model.Snapshot, changes []model.Change) (int, []string, error) {
	total := 0
	var unread []string
	for _, c := range changes {
		n, err := changeLines(s, c)
		switch {
		case errors.Is(err, model.ErrTooLarge):
			unread = append(unread, c.NewPath)
		case err != nil:
			return 0, nil, err
		default:
			total += n
		}
	}
	return total, unread, nil
}

// changeLines counts the lines one change brings: every line of an added
// file and, for a modified or renamed one, the lines of the new blob the
// old blob does not hold; a deletion, a link, a submodule, a binary file
// and a change that keeps its blob bring none.
func changeLines(s model.Snapshot, c model.Change) (int, error) {
	if c.Op == model.ChangeDelete || c.Mode == linkMode || c.Mode == gitlinkMode || (c.Op != model.ChangeAdd && c.Before == c.After) {
		return 0, nil
	}
	switch class := s.Class(c.NewPath); {
	case class == model.ClassUnreadable || class == model.ClassUnknown:
		return 0, fmt.Errorf("%w: the staged %s", model.ErrUnreadable, c.NewPath)
	case !class.IsText():
		return 0, nil
	}
	after, err := s.Read(c.NewPath)
	if err != nil {
		return 0, fmt.Errorf("read the staged %s: %w", c.NewPath, err)
	}
	if c.Op == model.ChangeAdd {
		return len(contentLines(after)), nil
	}
	before, err := model.ReadBlob(s, c.Before)
	if err != nil {
		return 0, fmt.Errorf("read what %s held before: %w", c.NewPath, err)
	}
	return added(before, after), nil
}

// added counts the lines of after that before does not hold, compared as a
// multiset, so a moved line is no new line and a repeated one counts once
// per copy it gains.
func added(before, after []byte) int {
	held := map[string]int{}
	for _, line := range contentLines(before) {
		held[line]++
	}
	n := 0
	for _, line := range contentLines(after) {
		if held[line] > 0 {
			held[line]--
			continue
		}
		n++
	}
	return n
}

// contentLines splits a file into its lines the way wc counts them, plus
// an unterminated last line.
func contentLines(content []byte) []string {
	if len(content) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
}
