package surface

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

const (
	preferTagCode = "prefer-tag"
	preferTagKey  = "S6819"
	roleAttribute = "role"
	tagInput      = "input"
)

// roleRow maps a role to its native element: how a message spells it and
// the tags that already are it.
type roleRow struct {
	role    string
	native  string
	accepts []string
}

// roleRows are the roles of jsx-a11y/prefer-tag-over-role that have one
// native element (M6.7 check 7); a role with none is never reported.
func roleRows() []roleRow {
	one := func(role, tag string) roleRow { return roleRow{role, "<" + tag + ">", []string{tag}} }
	return []roleRow{
		one("status", "output"),
		{"img", "<img>", []string{"img", "svg"}},
		one("button", "button"), one("link", "a"),
		{"heading", "<h1> to <h6>", []string{"h1", "h2", "h3", "h4", "h5", "h6"}},
		{"list", "<ul> or <ol>", []string{"ul", "ol"}},
		one("listitem", "li"), one("navigation", "nav"),
		one("banner", "header"), one("contentinfo", "footer"), one("main", "main"), one("article", "article"),
		one("complementary", "aside"), one("dialog", "dialog"), one("table", "table"), one("row", "tr"),
		one("cell", "td"), one("progressbar", "progress"), one("separator", "hr"), one("checkbox", tagInput),
		one("radio", tagInput),
		{"textbox", "<" + tagInput + ">", []string{tagInput, "textarea"}},
	}
}

// HTMLPreferTag reports a role on an element where a native element
// exists. It is feedback, because markup composed across Go templates
// cannot be judged with certainty, and fails a scan only under the strict
// rule set.
func HTMLPreferTag() model.Check {
	return check{
		spec:      spec("surface.html-prefer-tag"),
		languages: []detect.Language{detect.HTML, detect.JSX, detect.TSX},
		strict:    true,
		judge:     judgePreferTag,
	}
}

// judgePreferTag reads the markup of an HTML file and the elements of a
// JSX or TSX file.
func judgePreferTag(f source, _ model.Input) []model.Finding {
	if f.info.Language == detect.HTML {
		return htmlRoles(f)
	}
	return jsxRoles(f)
}

// htmlRoles judges the start tags of the markup.
func htmlRoles(f source) []model.Finding {
	var out []model.Finding
	for _, e := range markupElements(f.src, f.info.Regions) {
		if value, ok := e.attrs[roleAttribute]; ok {
			out = append(out, roleFinding(f, e.tag, value, e.line, "Web:"+preferTagKey)...)
		}
	}
	return out
}

// jsxRoles judges the elements of a JSX or TSX unit: a tag that starts
// with a lower case letter is markup, a component is not, and its role
// counts only when a string gives it.
func jsxRoles(f source) []model.Finding {
	tokens := significant(lexJS(f.src, 1, true))
	key := sonarKey(f.info.Language, preferTagKey)
	var out []model.Finding
	for i, t := range tokens {
		if t.kind != tokTag || t.text == "" || !isLower(t.text[0]) {
			continue
		}
		if value, ok := jsxRole(tokens, i); ok {
			out = append(out, roleFinding(f, t.text, value, t.line, key)...)
		}
	}
	return out
}

// jsxRole answers the string value of the role attribute of the element
// whose tag is at i, reading its attributes and skipping its expression
// containers.
func jsxRole(tokens []token, i int) (string, bool) {
	for j := i + 1; j < len(tokens); j++ {
		t := tokens[j]
		switch {
		case isPunctAt(tokens, j, "{"):
			j = closing(tokens, j)
		case t.kind == tokIdent && t.text == roleAttribute && isPunctAt(tokens, j+1, "="):
			if j+2 < len(tokens) && tokens[j+2].kind == tokString {
				return stringValue(tokens[j+2]), true
			}
			return "", false
		case t.kind == tokIdent, t.kind == tokString, isPunctAt(tokens, j, "="):
		default:
			return "", false
		}
	}
	return "", false
}

// roleFinding answers the finding of a role on a tag that is not its
// native element. A value a template splices is decided at serve time and
// is not judged.
func roleFinding(f source, tag, value string, line int, key string) []model.Finding {
	if strings.Contains(value, "{{") {
		return nil
	}
	words := strings.Fields(strings.ToLower(value))
	if len(words) == 0 {
		return nil
	}
	for _, row := range roleRows() {
		if row.role != words[0] || slices.Contains(row.accepts, tag) {
			continue
		}
		message := fmt.Sprintf("uses role=%q on <%s> where the native element %s exists (Sonar %s): use %s instead.",
			row.role, tag, row.native, key, row.native)
		return []model.Finding{f.finding(preferTagCode, line, message, key)}
	}
	return nil
}

func isLower(c byte) bool { return c >= 'a' && c <= 'z' }
