package profile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/runner"
)

// A clone lands in a hidden directory of the profiles directory: never a
// profile name, and on the filesystem of the store, so placing the bundle
// it holds is a rename.
const (
	fetchPattern = ".fetch-*"
	profilesMode = 0o700
	createCause  = "create %s: %v"
)

// Fetch clones src at its tag into a fresh directory under <config>/profiles/
// through runner.Run with git, env the user environment, and answers the
// directory; the caller removes it. A source without the tag as given is
// cloned at the other form of the same version (v1.2.3 for 1.2.3 and the
// reverse), because a refusal names the bare version of the pin (D8) and
// D9 reads both forms as one version. A clone that fails is ErrBundle
// naming the URL, the tag as given and git's last line, and leaves no
// directory behind.
func Fetch(ctx context.Context, configDir string, src Source, env []string) (string, error) {
	root := profilesRoot(configDir)
	if err := os.MkdirAll(root, profilesMode); err != nil {
		return "", bundlef(createCause, root, err)
	}
	dir, err := cloneAt(ctx, root, src, src.Tag, env)
	if err == nil || ctx.Err() != nil {
		return dir, err
	}
	if other, otherErr := cloneAt(ctx, root, src, otherTag(src.Tag), env); otherErr == nil {
		return other, nil
	}
	return "", err
}

// cloneAt clones src at tag into a fresh hidden directory of root; a clone
// that fails names the tag the person gave and removes its directory.
func cloneAt(ctx context.Context, root string, src Source, tag string, env []string) (string, error) {
	dir, err := os.MkdirTemp(root, fetchPattern)
	if err != nil {
		return "", bundlef("create a clone directory under %s: %v", root, err)
	}
	out, err := runner.Run(ctx, runner.Command{Name: "git", Args: cloneArgs(src.URL, tag, dir), Dir: root, Env: env})
	if err != nil {
		return "", errors.Join(bundlef("clone %s at %s: %v", src.URL, src.Tag, err), removeDir(dir))
	}
	if out.Code != 0 {
		return "", errors.Join(bundlef("clone %s at %s: %s", src.URL, src.Tag, lastLine(out.Stderr)), removeDir(dir))
	}
	return dir, nil
}

// otherTag is the other form of a version tag: v1.2.3 for 1.2.3 and the
// reverse.
func otherTag(tag string) string {
	if bare, found := strings.CutPrefix(tag, "v"); found {
		return bare
	}
	return "v" + tag
}

// cloneArgs clone one tag with its bytes as committed, because the digest
// is over the committed bytes and a line ending conversion would change
// it; the separator keeps a URL that starts with a dash from being read
// as an option.
func cloneArgs(url, tag, dir string) []string {
	return []string{
		"clone", "--quiet", "--depth", "1", "--branch", tag,
		"--config", "advice.detachedHead=false", "--config", "core.autocrlf=false",
		"--", url, dir,
	}
}

// lastLine is the last line git printed on its error stream, the one that
// names the cause.
func lastLine(stderr []byte) string {
	lines := strings.Split(strings.TrimSpace(string(stderr)), "\n")
	if last := strings.TrimSpace(lines[len(lines)-1]); last != "" {
		return last
	}
	return "git failed without a message"
}

// profilesRoot is the directory that holds every bundle of the config
// directory: the stamp path of an empty name is a file directly under it.
func profilesRoot(configDir string) string {
	return filepath.Dir(policy.StampPath(configDir, ""))
}

// removeDir removes a directory the lifecycle made, naming it on failure.
func removeDir(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove %s: %w", dir, err)
	}
	return nil
}
