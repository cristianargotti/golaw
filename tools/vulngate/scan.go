package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/vuln"
)

const scannerErrorFormat = "%w: %w"

// scanDependencies are the clock and the process runner of the scan mode,
// supplied by main, so the judgment of a stream never reads either itself.
type scanDependencies struct {
	now        time.Time
	readOutput func(context.Context, runner.Command, func(io.Reader) error) (runner.Output, error)
}

// scanFindings runs the pinned govulncheck in the working directory with
// the user environment and answers the reachable findings of a complete
// stream; every failure carries ErrScanner, because a scan that did not
// finish must never read as clean.
func scanFindings(ctx context.Context, deps scanDependencies) ([]vuln.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf(scannerErrorFormat, ErrScanner, err)
	}
	var data []byte
	command := runner.Command{
		Name: "govulncheck", Args: []string{"-format", "json", "./..."},
		Env: runner.UserEnv(os.Environ()),
	}
	out, err := deps.readOutput(ctx, command, func(r io.Reader) error {
		var readErr error
		data, readErr = readScanOutput(r)
		return readErr
	})
	if err != nil {
		return nil, fmt.Errorf(scannerErrorFormat, ErrScanner, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf(scannerErrorFormat, ErrScanner, err)
	}
	if out.Code != 0 {
		return nil, fmt.Errorf("%w: govulncheck exited %d: %s", ErrScanner, out.Code, strings.TrimSpace(string(out.Stderr)))
	}
	return pinnedFindings(data)
}

// readScanOutput keeps the stream up to the runner's bound and refuses a
// longer one, because a truncated stream could drop a reachable finding.
func readScanOutput(r io.Reader) ([]byte, error) {
	data := make([]byte, runner.MaxOutputBytes)
	size := 0
	for size < len(data) {
		n, err := r.Read(data[size:])
		size += n
		if errors.Is(err, io.EOF) {
			return data[:size], nil
		}
		if err != nil {
			return nil, fmt.Errorf("read govulncheck: %w", err)
		}
	}
	var next [1]byte
	if _, err := io.ReadFull(r, next[:]); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("read govulncheck: %w", err)
		}
		return nil, fmt.Errorf("%w: govulncheck output exceeds %d bytes", ErrScanner, runner.MaxOutputBytes)
	}
	return data, nil
}

// pinnedFindings reads the scanner version the stream itself reports, so
// the pin holds without a second process that would query the network.
func pinnedFindings(data []byte) ([]vuln.Finding, error) {
	version, err := vuln.Scanner(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf(scannerErrorFormat, ErrScanner, err)
	}
	tool, _ := runner.LookupTool("govulncheck")
	if !tool.Matches(version) {
		return nil, fmt.Errorf("%w: govulncheck version %q, expected %s: run golaw setup", ErrScanner, version, tool.Version)
	}
	found, err := vuln.Reachable(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf(scannerErrorFormat, ErrScanner, err)
	}
	return found, nil
}
