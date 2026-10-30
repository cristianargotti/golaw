package toolchain

import (
	"context"
	"fmt"
	"os"
)

// toolRuntime judges a PyPI tool by the interpreter its venv starts it
// with once that venv exists: the PATH python3 only builds the venv, and a
// repository may pin an older one there.
func (r Resolver) toolRuntime(ctx context.Context, t Tool, rt Runtime) RuntimeStatus {
	if t.Distribution == DistPyPI && r.Dir != "" && r.venvLive(t) {
		return r.probeRuntime(ctx, rt, VenvPython(r.Dir, r.GOOS), "the kit's virtual environment has")
	}
	return r.Runtime(ctx, rt)
}

// venvLive reports whether the interpreter a PyPI tool starts through still
// resolves. A venv links to the Python it was built on, so an upgrade that
// removes that Python leaves every script at its sha256 and unable to start.
func (r Resolver) venvLive(t Tool) bool {
	if t.Distribution != DistPyPI {
		return true
	}
	_, err := os.Stat(VenvPython(r.Dir, r.GOOS))
	return err == nil
}

func venvGone(t Tool, python string) Resolved {
	return missing(t, fmt.Sprintf("%s %s: the Python of its virtual environment at %s is gone: %s", t.Name, t.Version, python, fix))
}
