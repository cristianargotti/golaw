package main

import (
	"bytes"
	"context"
	"go/format"
	"os"
	"slices"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/apply"
	"github.com/cristianargotti/golaw/internal/gen/imports"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// The names of the mechanical fixes, as the feedback line lists them.
const (
	partFormat  = "go/format"
	partImports = "imports fixed"
	partLintFmt = "golangci-lint fmt"
)

// fixedFile is what the mechanical fixes did to one Go file: the fixes
// that changed it, the import lines they could not decide, the parser
// error that left it alone, or why its write was skipped.
type fixedFile struct {
	path           string
	parts, report  []string
	parse, skipped string
}

// fmtStage is an external formatter over the fixed bytes; false leaves the
// bytes as they were, because a tool that cannot run is never a finding.
type fmtStage func(ctx context.Context, content []byte) ([]byte, bool)

// fixFiles applies the certain fixes of A45 to each distinct path in order,
// with one import index for the event. ctx carries the event's external
// budget (A28), shared with the generators before and the linters after,
// so the hook answers within the host's limit however many files the
// event wrote.
func fixFiles(ctx context.Context, root, module string, snap model.Snapshot, paths []string, stage fmtStage) []fixedFile {
	if len(paths) == 0 {
		return nil
	}
	index := imports.Std().With(module, snap)
	var out []fixedFile
	for _, p := range slices.Compact(slices.Sorted(slices.Values(paths))) {
		if f, ok := fixFile(ctx, root, p, index, stage); ok {
			out = append(out, f)
		}
	}
	return out
}

// fixFile fixes one file and writes it only when its bytes differ, only
// over the bytes it read, so a concurrent edit is skipped and never lost.
// A file that does not parse is left as it is (M7.9.3 item 1).
func fixFile(ctx context.Context, root, path string, index imports.Index, stage fmtStage) (fixedFile, bool) {
	onDisk, err := os.ReadFile(source.OSPath(root, path))
	if err != nil {
		return fixedFile{}, false
	}
	f := fixedFile{path: path}
	formatted, err := format.Source(onDisk)
	if err != nil {
		f.parse = path + ":" + err.Error()
		return f, true
	}
	if !bytes.Equal(formatted, onDisk) {
		f.parts = append(f.parts, partFormat)
	}
	fix, err := imports.Apply(path, formatted, index)
	if err != nil {
		// Apply's parser error names the path and the line already.
		f.parse = err.Error()
		return f, true
	}
	f.report = fix.Report
	content := fix.Content
	if !bytes.Equal(content, formatted) {
		f.parts = append(f.parts, partImports)
	}
	// A spent event starts no formatter: the in process fixes still land.
	if stage != nil && ctx.Err() == nil {
		if staged, ok := stage(ctx, content); ok && !bytes.Equal(staged, content) {
			f.parts, content = append(f.parts, partLintFmt), staged
		}
	}
	if bytes.Equal(content, onDisk) {
		return f, len(f.report) > 0
	}
	return landFixed(root, f, content, onDisk), true
}

// landFixed writes the fixed bytes as a replacement bound to the bytes read
// (contract 2.6).
func landFixed(root string, f fixedFile, content, onDisk []byte) fixedFile {
	report, err := apply.Files(root, []gen.Write{{Path: f.path, Content: content, Mode: gen.WriteRegenerable, Base: source.ContentHash(onDisk)}})
	switch {
	case err != nil:
		f.parts, f.skipped = nil, err.Error()
	case len(report.Skipped) > 0:
		f.parts, f.skipped = nil, report.Skipped[0].Cause
	}
	return f
}
