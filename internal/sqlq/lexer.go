// Package sqlq translates a documented subset of SQL SELECT into MongoDB
// find queries or aggregation pipelines. See docs/REFERENCE.md "SQL query" for the
// grammar; anything outside it is rejected with a positioned error.
package sqlq

import (
	"fmt"
	"strings"
	"unicode"
)

type tokKind int

const (
	tEOF tokKind = iota
	tIdent
	tQuotedIdent
	tString
	tNumber
	tOp    // = != <> < <= > >=
	tPunct // ( ) , * . ;
)

type token struct {
	kind tokKind
	text string // identifiers keep their case; keywords compare case-insensitively
	pos  int    // byte offset
}

// Error is a SQL error at a position.
type Error struct {
	Line, Col int
	Msg       string
}

func (e *Error) Error() string { return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Col, e.Msg) }

func errAt(src string, pos int, format string, args ...any) *Error {
	line, col := 1, 1
	for i, r := range src {
		if i >= pos {
			break
		}
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return &Error{Line: line, Col: col, Msg: fmt.Sprintf(format, args...)}
}

func lex(src string) ([]token, error) {
	var toks []token
	i := 0
	for i < len(src) {
		c := rune(src[i])
		switch {
		case unicode.IsSpace(c):
			i++
		case c == '-' && i+1 < len(src) && src[i+1] == '-':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return nil, errAt(src, i, "unterminated comment")
			}
			i += end + 4
		case c == '\'':
			start := i
			var b strings.Builder
			i++
			for {
				if i >= len(src) {
					return nil, errAt(src, start, "unterminated string literal")
				}
				if src[i] == '\'' {
					if i+1 < len(src) && src[i+1] == '\'' { // '' escape
						b.WriteByte('\'')
						i += 2
						continue
					}
					i++
					break
				}
				b.WriteByte(src[i])
				i++
			}
			toks = append(toks, token{tString, b.String(), start})
		case c == '"' || c == '`':
			start := i
			end := strings.IndexByte(src[i+1:], src[i])
			if end < 0 {
				return nil, errAt(src, start, "unterminated quoted identifier")
			}
			toks = append(toks, token{tQuotedIdent, src[i+1 : i+1+end], start})
			i += end + 2
		case unicode.IsDigit(c) || (c == '.' && i+1 < len(src) && unicode.IsDigit(rune(src[i+1]))) ||
			(c == '-' && i+1 < len(src) && (unicode.IsDigit(rune(src[i+1])) || src[i+1] == '.')):
			start := i
			if c == '-' {
				i++
			}
			for i < len(src) && (unicode.IsDigit(rune(src[i])) || src[i] == '.') {
				i++
			}
			if i < len(src) && (src[i] == 'e' || src[i] == 'E') {
				i++
				if i < len(src) && (src[i] == '+' || src[i] == '-') {
					i++
				}
				for i < len(src) && unicode.IsDigit(rune(src[i])) {
					i++
				}
			}
			toks = append(toks, token{tNumber, src[start:i], start})
		case c == '_' || c == '$' || unicode.IsLetter(c):
			start := i
			for i < len(src) {
				r := rune(src[i])
				if r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r) || r >= 0x80 {
					i++
					continue
				}
				break
			}
			toks = append(toks, token{tIdent, src[start:i], start})
		case strings.ContainsRune("=<>!", c):
			start := i
			op := string(c)
			if i+1 < len(src) {
				two := src[i : i+2]
				if two == "<=" || two == ">=" || two == "<>" || two == "!=" {
					op = two
				}
			}
			if op == "!" {
				return nil, errAt(src, i, "unexpected '!'")
			}
			i += len(op)
			toks = append(toks, token{tOp, op, start})
		case strings.ContainsRune("(),*.;", c):
			toks = append(toks, token{tPunct, string(c), i})
			i++
		case c == '+' || c == '/' || c == '%' || c == '|':
			return nil, errAt(src, i, "arithmetic and string operators are not supported")
		default:
			return nil, errAt(src, i, "unexpected character %q", c)
		}
	}
	toks = append(toks, token{tEOF, "", len(src)})
	return toks, nil
}
