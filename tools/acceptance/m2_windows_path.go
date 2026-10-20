package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	m2RecordersVariable = "GOLAW_M2_RECORDERS"
	m2FixtureMain       = "main.go"
)

const m2WindowsRecorderProbe = `recorders=$(cd "$1" && pwd) || exit 90
for tool in make awk tar; do
  resolved=$(command -v "$tool") || exit 91
  case "$resolved" in
    "$recorders/$tool"|"$recorders/$tool.exe") ;;
    *)
      printf '%s\n' "recorder displaced: $tool: $resolved" >&2
      exit 92
      ;;
  esac
done`

func windowsPathM2(env, directories []string) []string {
	env = slices.DeleteFunc(slices.Clone(env), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(key, envPath)
	})
	return append(env, pathAssignment+strings.Join(directories, string(os.PathListSeparator)))
}

func (s *suite) m2WindowsEnvironment(ctx context.Context, repo string, env []string) ([]string, error) {
	shell, err := s.lookup("sh")
	if err != nil {
		return nil, fmt.Errorf("locate Windows shell: %w", err)
	}
	if shellErr := m2RequireExecutable(shell); shellErr != nil {
		return nil, shellErr
	}
	scanner, err := resolveExecutable(m2Scanner, env)
	if err != nil {
		return nil, err
	}
	recorders, err := s.m2BuildWindowsRecorders(ctx, repo, env)
	if err != nil {
		return nil, err
	}
	dirs := []string{
		recorders, filepath.Dir(s.binary), filepath.Dir(scanner), s.m2.Bin,
		filepath.Dir(s.goTool), filepath.Dir(s.git), filepath.Dir(shell),
	}
	env = windowsPathM2(env, m2DistinctDirectories(dirs))
	env = slices.DeleteFunc(env, func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(key, m2RecordersVariable)
	})
	return append(env, m2RecordersVariable+"="+recorders), nil
}

func (s *suite) m2BuildWindowsRecorders(ctx context.Context, repo string, env []string) (string, error) {
	body, err := readFile(filepath.Join(s.root, dirTools, dirAcceptance, m2Testdata, "m2", "path_recorder.go.txt"))
	if err != nil {
		return "", err
	}
	source := filepath.Join(s.base, "windows-recorder-source", m2FixtureMain)
	if writeErr := writeFile(source, body, fileMode); writeErr != nil {
		return "", writeErr
	}
	dir := filepath.Join(s.base, "windows-recorders")
	if mkdirErr := os.MkdirAll(dir, dirMode); mkdirErr != nil {
		return "", fmt.Errorf("create Windows recorder directory: %w", mkdirErr)
	}
	program := filepath.Join(dir, "path-recorder.exe")
	got := s.execute(ctx, command{
		program: s.goTool, args: []string{goBuild, m2Trimpath, "-o", program, source}, dir: repo, env: env,
	})
	if runErr := success(got); runErr != nil {
		return "", runErr
	}
	body, err = readFile(program)
	if err != nil {
		return "", err
	}
	if err := m2CopyWindowsRecorders(dir, body); err != nil {
		return "", err
	}
	return dir, nil
}

func m2CopyWindowsRecorders(dir string, body []byte) error {
	for _, name := range []string{"make.exe", "awk.exe", "tar.exe"} {
		if err := writeFile(filepath.Join(dir, name), body, execMode); err != nil {
			return err
		}
	}
	return nil
}

func (s *suite) m2WindowsProbe(ctx context.Context, shell, repo string, env []string) error {
	dir := environmentValue(env, m2RecordersVariable)
	if dir == "" {
		return fmt.Errorf("native Windows recorder directory is absent: %w", ErrAcceptance)
	}
	return expect(s.execute(ctx, command{
		program: shell, args: []string{"-c", m2WindowsRecorderProbe, "m2-probe", dir}, dir: repo, env: env,
	}), wantAllow)
}

func m2WindowsRecordersQuiet(env []string) error {
	dir := environmentValue(env, m2RecordersVariable)
	if dir == "" {
		return fmt.Errorf("native Windows recorder directory is absent: %w", ErrAcceptance)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("inspect Windows recorder markers: %w", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".started") {
			return fmt.Errorf("native Windows recorder was started: %s: %w", entry.Name(), ErrAcceptance)
		}
	}
	return nil
}
