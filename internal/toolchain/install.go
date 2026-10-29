package toolchain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/surface"
)

// Fetcher opens one download; setup passes the person's HTTP client, and a
// test a local server, so the installer itself never chooses a network.
type Fetcher func(ctx context.Context, url string) (io.ReadCloser, error)

// InstallOptions is one run of golaw setup --surface.
type InstallOptions struct {
	Dir       string // policy.Dirs.Tools; empty is ErrNoToolsDir
	GOOS      string // the target the layout and the assets name
	GOARCH    string // the target the layout and the assets name
	Languages []surface.Language
	All       bool // every tool of the table, whatever the repository holds
	Fetch     Fetcher
	// Run starts npm, pip and the venv; nil runs them through runner.Run.
	Run func(context.Context, runner.Command) (runner.Output, error)
	// Env is the person's whole environment, so the proxies and registry
	// settings of the machine reach npm and pip.
	Env []string
	Out io.Writer // the pt-BR lines; nil discards them
	// Tools replaces the table when set, so a test installs synthetic
	// tools a local server serves instead of the real pins.
	Tools []Tool
}

// MaxAssetBytes is far above any pinned asset, so a download or an archive
// member that grows past it is refused before it fills the memory.
const MaxAssetBytes = 256 << 20

// Install places and verifies the selected tools and answers the receipt
// of each one installed or already in place. A tool that cannot install is
// one error of the joined answer, never a stop for the others; a second
// run downloads nothing it already holds.
func Install(ctx context.Context, o InstallOptions) ([]Receipt, error) {
	if o.Dir == "" {
		return nil, ErrNoToolsDir
	}
	if o.Run == nil {
		o.Run = runner.Run
	}
	if o.Out == nil {
		o.Out = io.Discard
	}
	tools, err := o.selected()
	if err != nil {
		return nil, fmt.Errorf("install the surface tools: %w", err)
	}
	var receipts []Receipt
	var errs []error
	binaries, npm, pypi := byDistribution(tools)
	for i := range binaries {
		rec, err := installBinary(ctx, o, binaries[i])
		if err != nil {
			errs = append(errs, err)
			continue
		}
		receipts = append(receipts, rec)
	}
	for _, group := range []struct {
		tools   []Tool
		install func(context.Context, InstallOptions, []Tool) ([]Receipt, error)
	}{{npm, installNPM}, {pypi, installPyPI}} {
		if len(group.tools) == 0 {
			continue
		}
		recs, err := group.install(ctx, o, group.tools)
		receipts = append(receipts, recs...)
		errs = append(errs, err)
	}
	return receipts, errors.Join(errs...)
}

// selected answers the tools of the run in name order.
func (o InstallOptions) selected() ([]Tool, error) {
	tools := slices.Clone(o.Tools)
	if tools == nil {
		table, err := Table()
		if err != nil {
			return nil, err
		}
		tools = table
	}
	if !o.All {
		tools = slices.DeleteFunc(tools, func(t Tool) bool { return !lints(t, o.Languages) })
	}
	slices.SortStableFunc(tools, func(a, b Tool) int { return strings.Compare(a.Name, b.Name) })
	return tools, nil
}

// lints reports whether t lints any of langs.
func lints(t Tool, langs []surface.Language) bool {
	return slices.ContainsFunc(t.Languages, func(l surface.Language) bool { return slices.Contains(langs, l) })
}

// byDistribution splits the tools by how they install, keeping the order.
func byDistribution(tools []Tool) ([]Tool, []Tool, []Tool) {
	var binaries, npm, pypi []Tool
	for i := range tools {
		switch tools[i].Distribution {
		case DistBinary:
			binaries = append(binaries, tools[i])
		case DistNPM:
			npm = append(npm, tools[i])
		case DistPyPI:
			pypi = append(pypi, tools[i])
		}
	}
	return binaries, npm, pypi
}

// say writes one line of the person's channel; a closed output is an
// error, because the person would not see what was installed.
func say(out io.Writer, format string, args ...any) error {
	if _, err := fmt.Fprintf(out, format, args...); err != nil {
		return fmt.Errorf("write the setup line: %w", err)
	}
	return nil
}
