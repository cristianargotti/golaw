package surface

import (
	"bytes"
	"strings"
)

// RegionKind says what an HTML region holds.
type RegionKind int

// The region kinds of M6.3.1.
const (
	RegionScript RegionKind = iota + 1 // a <script> body without src and with no type or a JavaScript type
	RegionStyle                        // a <style> body
)

// Region is the body of one inline script or style of an HTML file. The
// body leaves out the line break right after the opening tag and the
// spaces before a closing tag that starts its own line, so Lines counts
// the lines of code and a script of exactly 40 lines is not 41.
type Region struct {
	Kind      RegionKind
	Start     int  // byte offset of the first byte of the body
	End       int  // byte offset one past the last byte of the body
	StartLine int  // 1-based line of Start
	EndLine   int  // 1-based line of the last byte of the body
	Spliced   bool // the body holds a Go template action "{{"
	Module    bool // type="module"
}

// Lines answers the lines the body spans, 0 for an empty body.
func (r Region) Lines() int {
	if r.End <= r.Start {
		return 0
	}
	return r.EndLine - r.StartLine + 1
}

// The markup the scanner recognizes.
const (
	commentOpen  = "<!--"
	commentClose = "-->"
	scriptTag    = "script"
	styleTag     = "style"
)

// Regions answers the script and style regions of an HTML file in file
// order. A tag inside a comment is not a region, and the body of a script
// or a style is skipped whole, so a tag a string inside it spells is not
// one either.
func Regions(content []byte) []Region {
	var out []Region
	for i := 0; i < len(content); {
		next := bytes.IndexByte(content[i:], '<')
		if next < 0 {
			break
		}
		i += next
		switch {
		case hasPrefixFold(content[i:], commentOpen):
			i = afterComment(content, i)
		case tagAt(content, i, scriptTag):
			region, after, found := element(content, i, scriptTag)
			if found {
				out = append(out, region)
			}
			i = after
		case tagAt(content, i, styleTag):
			region, after, _ := element(content, i, styleTag)
			out = append(out, region)
			i = after
		default:
			i++
		}
	}
	return out
}

// afterComment answers the offset after the comment that opens at i, the
// end of the content when it never closes.
func afterComment(content []byte, i int) int {
	end := bytes.Index(content[i+len(commentOpen):], []byte(commentClose))
	if end < 0 {
		return len(content)
	}
	return i + len(commentOpen) + end + len(commentClose)
}

// element reads the element whose opening tag starts at i: its region, the
// offset after its closing tag, and whether it is a region at all (a
// script with src or another type is not).
func element(content []byte, i int, name string) (Region, int, bool) {
	attributes, open := openTag(content, i+1+len(name))
	closing := closeTag(content, open, name)
	start, end := trimBody(content, open, closing)
	region := Region{
		Kind: RegionStyle, Start: start, End: end,
		StartLine: lineAt(content, start), EndLine: lineAt(content, max(start, end-1)),
		Spliced: bytes.Contains(content[start:end], []byte("{{")),
	}
	after := min(len(content), closing+len("</")+len(name))
	if name == styleTag {
		return region, after, true
	}
	kind := strings.ToLower(strings.TrimSpace(attributes["type"]))
	region.Kind, region.Module = RegionScript, kind == "module"
	_, served := attributes["src"]
	return region, after, !served && javaScriptType(kind)
}

// trimBody drops the line break that follows the opening tag and the
// spaces and tabs before a closing tag that starts its own line.
func trimBody(content []byte, start, end int) (int, int) {
	if bytes.HasPrefix(content[start:end], []byte("\r\n")) {
		start += 2
	} else if bytes.HasPrefix(content[start:end], []byte("\n")) {
		start++
	}
	tail := end
	for tail > start && (content[tail-1] == ' ' || content[tail-1] == '\t') {
		tail--
	}
	if tail > start && content[tail-1] == '\n' {
		end = tail
	}
	return start, end
}

// lineAt answers the 1-based line of an offset.
func lineAt(content []byte, offset int) int {
	return 1 + bytes.Count(content[:offset], []byte("\n"))
}
