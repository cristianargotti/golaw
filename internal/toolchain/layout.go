package toolchain

import "path/filepath"

// develVersion names the configuration of a development build, which has
// no release to name (A10).
const develVersion = "devel"

const goosWindows = "windows"

func exe(goos string) string {
	if goos == goosWindows {
		return ".exe"
	}
	return ""
}

// BinPath is where a binary tool lives: one directory per version and
// platform, so two kit versions on one machine never share an executable.
func BinPath(toolsDir string, t Tool, goos, goarch string) string {
	return filepath.Join(toolsDir, "bin", t.Name, t.Version, Platform(goos, goarch), t.Name+exe(goos))
}

// NodeDir is the npm installation of the embedded lock, named by the lock
// so a new kit version installs beside the old one instead of over it.
func NodeDir(toolsDir string) string {
	return filepath.Join(toolsDir, runtimeNode, lockID(npmLock))
}

// PythonDir is the virtual environment of the embedded PyPI lock.
func PythonDir(toolsDir string) string {
	return filepath.Join(toolsDir, runtimePython, lockID(pypiLock), "venv")
}

// ConfigDir is where one kit configuration set is materialized; the
// repository never receives a kit configuration (M6.4.1).
func ConfigDir(toolsDir, kitVersion, set string) string {
	if kitVersion == "" {
		kitVersion = develVersion
	}
	return filepath.Join(toolsDir, "config", kitVersion, set)
}

// venvExec is an executable of a virtual environment, which Windows keeps
// under Scripts with its extension.
func venvExec(venv, name, goos string) string {
	if goos == goosWindows {
		return filepath.Join(venv, "Scripts", name+".exe")
	}
	return filepath.Join(venv, "bin", name)
}

// VenvPython is the interpreter of the kit's virtual environment, the one
// every PyPI tool of the lock starts through.
func VenvPython(toolsDir, goos string) string {
	return venvExec(PythonDir(toolsDir), runtimePython, goos)
}

// ExecPath is the executable the runner starts for a tool.
func ExecPath(toolsDir string, t Tool, goos, goarch string) string {
	switch t.Distribution {
	case DistBinary:
		return BinPath(toolsDir, t, goos, goarch)
	case DistNPM:
		name := t.Bin
		if goos == goosWindows {
			name += ".cmd"
		}
		return filepath.Join(NodeDir(toolsDir), "node_modules", ".bin", name)
	case DistPyPI:
		if t.Script && goos == goosWindows {
			return filepath.Join(PythonDir(toolsDir), "Scripts", t.Bin)
		}
		return venvExec(PythonDir(toolsDir), t.Bin, goos)
	}
	return ""
}
