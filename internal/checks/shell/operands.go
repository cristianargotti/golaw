package shell

import (
	"regexp"
	"strings"
)

// redirection matches the redirection operator a word starts with, its
// file descriptor or the PowerShell stream star included; the file it
// names follows in the same word or, after a bare operator, in the next.
var redirection = regexp.MustCompile(`^(?:\d+|\*)?[<>][<>&|]*`)

// The commands of the tools that the reading of their packages names.
const (
	commandRun  = "run"
	commandTest = "test"
)

// flagKind is what a flag of a Go tool does with the word after it.
type flagKind int

const (
	flagAlone   flagKind = iota // a switch, or a flag whose value is joined by =
	flagValue                   // the next word is its value
	flagUnknown                 // a flag the program does not take
)

// The flags of the go tool, those of its build, list, get, clean,
// generate, vet and test commands together: the switches, and the flags
// that take the next word as their value. A test binary flag is also taken
// under its test. prefix.
const (
	goSwitches = " a artifacts asan benchmem buildvcs c cache compiled cover deps e export failfast find fullpath " +
		"fuzzcache i json linkshared m modcache modcacherw msan n r race retracted short t test testcache tool " +
		"trimpath u v versions work x "
	goValues = " C asmflags bench benchtime blockprofile blockprofilerate buildmode compiler count covermode coverpkg " +
		"coverprofile cpu cpuprofile exec f fix fuzz fuzzminimizetime fuzztime gccgoflags gcflags installsuffix " +
		"ldflags list memprofile memprofilerate mod modfile mutexprofile mutexprofilefraction o outputdir overlay " +
		"p parallel pgo pkgdir reuse run shuffle skip tags timeout toolexec trace vet vettool "
)

// lintSwitches are the switches of the linters that take package patterns;
// any other flag of a linter takes the next word as its value.
const lintSwitches = " abspath allow-parallel-runners allow-serial-runners asserts blank enable-audit " +
	"exclude-generated excludeonly fast fast-only fix h help ignoregenerated ignoretests json list-checks matrix " +
	"merge n new no-config no-fail nosec print-resources-usage quiet set_exit_status show-ignored show-stats sort " +
	"stdout terse test tests track-suppressions uniq-by-line v verbose version whole-files "

// operandScan reads the words after the subcommand of a Go tool and marks
// the ones the tool reads as packages.
type operandScan struct {
	marks  []bool
	flag   func(name string) flagKind
	run    bool // go run: the first operand is the package, the rest its arguments
	test   bool // go test: the packages are the first run of operands, as go test reads them
	seen   bool // a package was read
	closed bool // go test: a flag after the packages or a flag it does not take closed their list
}

// packageWords marks the words of a segment its program reads as Go
// package patterns: the operands of a package command of the go tool and
// of the linters that take patterns, never a redirection or its file, the
// value of a flag, an argument go run or go test passes on to the program,
// or a word of a command that reads files, such as go tool; the value of
// -coverpkg is a list of patterns too.
func packageWords(seg segment) []bool {
	marks := make([]bool, len(seg))
	program := unwrap(seg)
	s, from, ok := scanFor(program)
	if ok {
		s.marks = marks[len(seg)-len(program):]
		s.scan(program, from)
	}
	return marks
}

// scanFor answers how the words of a program are read for packages and
// where its operands may start; ok is false for a program or a command
// that reads no package pattern.
func scanFor(words []string) (operandScan, int, bool) {
	if len(words) < 2 {
		return operandScan{}, 0, false
	}
	switch base(words[0]) {
	case "go":
		at := 1
		if words[1] == "-C" {
			at = 3
		}
		if at >= len(words) || !goPackageCommand(words[at]) {
			return operandScan{}, 0, false
		}
		return operandScan{flag: goFlag, run: words[at] == commandRun, test: words[at] == commandTest}, at + 1, true
	case "golangci-lint":
		return operandScan{flag: lintFlag}, 2, words[1] == commandRun
	case "staticcheck", "govulncheck", "gosec", "revive", "errcheck":
		return operandScan{flag: lintFlag}, 1, true
	default:
		return operandScan{}, 0, false
	}
}

// goPackageCommand reports whether a go command reads its operands as
// packages.
func goPackageCommand(command string) bool {
	switch command {
	case "build", "clean", "fix", "fmt", "generate", "get", "install", "list", commandRun, commandTest, "vet":
		return true
	default:
		return false
	}
}

// scan marks the operands among the words from the given index: a
// redirection and the file after a bare operator are skipped, a flag is
// read by flagWord, and a word after the closed list of go test passes to
// the test binary.
func (s *operandScan) scan(words []string, from int) {
	for i := from; i < len(words); i++ {
		word := words[i]
		switch op := redirection.FindString(word); {
		case op != "":
			if op == word {
				i++
			}
		case len(word) > 1 && word[0] == '-':
			next, stop := s.flagWord(words, i)
			if stop {
				return
			}
			i = next
		case s.closed:
		default:
			s.marks[i], s.seen = true, true
			if s.run {
				return
			}
		}
	}
}

// flagWord reads the flag at i and answers the index of the last word it
// takes, or stop when no package can follow it: the end of the flags, the
// arguments of a test binary or a flag the program does not take, which
// go test passes on to the binary instead, closing its list of packages.
func (s *operandScan) flagWord(words []string, i int) (int, bool) {
	name, _, joined := strings.Cut(strings.TrimLeft(words[i], "-"), "=")
	kind := s.flag(name)
	if name == "" || name == "args" || kind == flagUnknown && !s.test {
		return i, true
	}
	s.closed = s.closed || s.test && (s.seen || kind == flagUnknown)
	switch {
	case name == "coverpkg" && joined:
		s.marks[i] = true
	case name == "coverpkg" && i+1 < len(words):
		s.marks[i+1] = true
		return i + 1, false
	case kind == flagValue && !joined:
		return i + 1, false
	}
	return i, false
}

// goFlag is what a flag of the go tool takes; a dotted name, the flag of a
// vet analyzer, takes a value.
func goFlag(name string) flagKind {
	name = strings.TrimPrefix(name, "test.")
	switch {
	case strings.Contains(goSwitches, " "+name+" "):
		return flagAlone
	case strings.Contains(goValues, " "+name+" ") || strings.Contains(name, "."):
		return flagValue
	default:
		return flagUnknown
	}
}

// lintFlag is what a flag of a linter takes.
func lintFlag(name string) flagKind {
	if strings.Contains(lintSwitches, " "+name+" ") {
		return flagAlone
	}
	return flagValue
}
