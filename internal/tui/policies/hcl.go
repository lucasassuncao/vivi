package policies

import (
	"strings"

	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// highlight colours a policy document with a deliberately small lexer, not a
// parser: a wrong guess costs nothing because the text is shown verbatim, while
// a real parser would fail on a policy Vault accepts.
func highlight(st ui.Styles, doc string) string {
	lines := strings.Split(doc, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, highlightLine(st, line))
	}
	return strings.Join(out, "\n")
}

// keywords are the identifiers worth picking out of a policy: the block names
// and the fields an auditor actually reads.
var keywords = map[string]bool{
	"path":                     true,
	"capabilities":             true,
	"allowed_parameters":       true,
	"denied_parameters":        true,
	"required_parameters":      true,
	"min_wrapping_ttl":         true,
	"max_wrapping_ttl":         true,
	"allowed_response_headers": true,
	"control_group":            true,
	"identity":                 true,
	"true":                     true,
	"false":                    true,
}

func highlightLine(st ui.Styles, line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
		return st.HCLComment.Render(line)
	}

	var b strings.Builder
	runes := []rune(line)

	for i := 0; i < len(runes); {
		switch c := runes[i]; {
		case c == '"':
			// Strings run to the next unescaped quote, or to end of line on an
			// unterminated one.
			j := i + 1
			for j < len(runes) && (runes[j] != '"' || runes[j-1] == '\\') {
				j++
			}
			if j < len(runes) {
				j++
			}
			b.WriteString(st.HCLString.Render(string(runes[i:j])))
			i = j

		case c == '{' || c == '}' || c == '[' || c == ']' || c == '=' || c == ',':
			b.WriteString(st.HCLPunct.Render(string(c)))
			i++

		case isIdentRune(c):
			j := i
			for j < len(runes) && isIdentRune(runes[j]) {
				j++
			}
			word := string(runes[i:j])
			switch {
			case keywords[word]:
				b.WriteString(st.HCLKeyword.Render(word))
			case isNumber(word):
				b.WriteString(st.HCLNumber.Render(word))
			default:
				b.WriteString(word)
			}
			i = j

		default:
			b.WriteRune(c)
			i++
		}
	}

	return b.String()
}

func isIdentRune(r rune) bool {
	return r == '_' || r == '-' ||
		(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
