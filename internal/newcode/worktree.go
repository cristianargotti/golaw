package newcode

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

// WorktreeLines names, per module-relative path, the lines the worktree
// adds to the Go files since the merge base of base and HEAD: the commits,
// the index and the unstaged edits as diff-index compares them, and every
// line of an untracked Go file. golaw check reads it, because an agent
// measures the change it is still writing, before any commit.
func WorktreeLines(ctx context.Context, root, base string) (map[string][]int, error) {
	fork, err := MergeBase(ctx, root, base)
	if err != nil {
		return nil, err
	}
	out, err := plumbing(ctx, root, "diff-index", "-p", "--unified=0", "--no-renames", "--text",
		"--no-color", "--no-ext-diff", "--src-prefix=a/", "--dst-prefix="+newPrefix,
		fork, "--", goPathspec)
	if err == nil {
		err = bounded(out)
	}
	if err != nil {
		return nil, err
	}
	lines, err := parseDiff(string(out.Stdout))
	if err != nil {
		return nil, err
	}
	return lines, untrackedLines(ctx, root, lines)
}

// plumbing runs one git command that must exit 0.
func plumbing(ctx context.Context, root string, args ...string) (runner.Output, error) {
	out, err := git(ctx, root, args...)
	if err == nil && out.Code != 0 {
		err = failure(args[0], out)
	}
	return out, err
}

// untrackedLines adds every line of each untracked Go file, all of which
// are new; ls-files answers the paths from the repository root.
func untrackedLines(ctx context.Context, root string, lines map[string][]int) error {
	out, err := plumbing(ctx, root, "ls-files", "--others", "--exclude-standard", "--full-name", "-z", "--", goPathspec)
	if err != nil {
		return err
	}
	for name := range strings.SplitSeq(string(out.Stdout), "\x00") {
		if name == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name))) //nolint:gosec // git names the untracked file under the repository root
		if err != nil {
			continue // a file removed since the listing adds no line
		}
		count := bytes.Count(data, []byte("\n"))
		if len(data) > 0 && data[len(data)-1] != '\n' {
			count++
		}
		for number := 1; number <= count; number++ {
			lines[name] = append(lines[name], number)
		}
	}
	return nil
}
