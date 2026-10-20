package main

import "strings"

const m2Module = "module example.invalid/m2\n\ngo 1.27\n"

const m2Safe = "first line\nsecond line\n"

const m2Changed = "first changed line\nsecond line\n"

func m2RedText() string {
	return "first changed line\nsecond \u2014 line\n"
}

// m2GoLines is a Go file of exactly n lines, the big.go of the length rows.
func m2GoLines(n int) string {
	return "package m2\n" + strings.Repeat("// Frozen line.\n", n-1)
}

const (
	m2PathDirty      = "dirty.txt"
	m2PathNew        = "new.txt"
	m2PathOld        = "old.txt"
	m2PathBig        = "big.go"
	m2PathExtra      = "extra.txt"
	m2AlternateIndex = "alternate-index"
	m2Deletion       = "deletion"
	m2Rename         = "rename"
	m2FileLineLimit  = 300
	m2RenameLines    = 20
)

type m2FixtureFiles struct {
	head     map[string]string
	index    map[string]string
	checkout map[string]string
}

func m2StagedFiles(name string) m2FixtureFiles {
	f := m2FixtureFiles{
		head:     map[string]string{moduleFile: m2Module, pathNote: m2Safe},
		index:    map[string]string{pathNote: m2RedText()},
		checkout: map[string]string{pathNote: m2Changed},
	}
	if strings.HasSuffix(name, "-near") {
		f.index[pathNote], f.checkout[pathNote] = m2Changed, m2RedText()
	}
	family, _, _ := strings.Cut(name, "-")
	switch family {
	case "partial":
		if name == "partial-red" {
			f.checkout[pathNote] = "first line\nsecond \u2014 line\n"
		}
	case m2Deletion:
		f = m2DeletionFiles(f, name)
	case m2Rename:
		f = m2RenameFiles(f, name)
	case "dirty":
		f = m2DirtyFiles(f, name)
	case "length":
		f = m2LengthFiles(f, name)
	case categoryPolicy:
		f.index, f.checkout = map[string]string{}, map[string]string{}
	}
	return f
}

func m2DeletionFiles(f m2FixtureFiles, name string) m2FixtureFiles {
	// Distinct deleted bytes keep Git from pairing the addition as an unchanged rename.
	f.head[pathNote] = "removed record\nold \u2014 content\n"
	f.index = map[string]string{m2PathNew: m2RedText()}
	f.checkout = map[string]string{m2PathNew: m2Changed}
	if name == "deletion-near" {
		f.head[pathNote] = m2RedText()
		f.index = map[string]string{}
		f.checkout = map[string]string{pathNote: m2RedText()}
	}
	return f
}

func m2RenameFiles(f m2FixtureFiles, name string) m2FixtureFiles {
	prefix := strings.Repeat(m2Safe, m2RenameLines)
	delete(f.head, pathNote)
	f.head[m2PathOld] = prefix + m2Safe
	f.index = map[string]string{m2PathNew: prefix + m2RedText()}
	f.checkout = map[string]string{m2PathNew: prefix + m2Changed}
	if name == "rename-near" {
		f.head[m2PathOld], f.checkout[m2PathNew] = prefix+m2RedText(), prefix+m2RedText()
	}
	return f
}

func m2DirtyFiles(f m2FixtureFiles, name string) m2FixtureFiles {
	f.head[m2PathDirty] = m2Safe
	f.index = map[string]string{m2PathDirty: m2RedText()}
	f.checkout = map[string]string{m2PathDirty: m2Safe}
	if name == "dirty-near" {
		f.index = map[string]string{pathNote: m2Changed}
		f.checkout = map[string]string{
			pathNote: m2Changed, m2PathDirty: m2RedText(), m2PathExtra: m2RedText(),
		}
	}
	return f
}

func m2LengthFiles(f m2FixtureFiles, name string) m2FixtureFiles {
	n := 300
	if name == "length-near" {
		n = 320
	}
	f.head[m2PathBig] = m2GoLines(n)
	f.index, f.checkout = map[string]string{m2PathBig: m2GoLines(n + 1)},
		map[string]string{m2PathBig: m2GoLines(n + 1)}
	return f
}
