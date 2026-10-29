package toolchain

import (
	"fmt"
	"os"
)

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
