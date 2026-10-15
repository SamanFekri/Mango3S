// Package shell implements a basic IntelliShell-style command runner. It is
// NOT a JavaScript engine: it understands the mongosh command forms people use
// day to day (show/use/help, db.<coll>.<method>(...) with cursor modifiers,
// db.<method>(...)) with arguments in shell literal syntax.
package shell

import (
	"fmt"
	"strings"

	"mango3s/internal/mql"
)

type segment struct {
	name string
	call bool
	args []any
}

type statement struct {
	text string
	kind string // show | use | help | db
	word string
	segs []segment
}

// Parse splits a script into statements. Statements are separated by ';' or
// simply follow each other; a line starting with '.' continues the previous
// call chain, as in mongosh.
func Parse(script string) ([]statement, error) {
	p := mql.NewParser(script)
	var out []statement
	for !p.EOF() {
		if p.Consume(';') {
			continue
		}
		st, err := parseStatement(p)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}

func parseStatement(p *mql.Parser) (statement, error) {
	p.SkipSpace()
	start := p.Pos()
	id, ok := p.Ident()
	if !ok {
		return statement{}, p.Errorf("expected a command such as db.collection.find(), show dbs or use <db>")
	}
	st := statement{}
	switch id {
	case "show", "use":
		st.kind = id
		st.word = p.Word()
		if st.word == "" {
			return statement{}, p.Errorf("%s requires an argument", id)
		}
	case "help":
		st.kind = "help"
	case "db":
		st.kind = "db"
		segs, err := parseChain(p)
		if err != nil {
			return statement{}, err
		}
		st.segs = segs
	case "it":
		return statement{}, p.Errorf(`"it" is not supported; use .skip(n) to page through results`)
	case "var", "let", "const", "function", "for", "while", "if", "print", "printjson", "load", "require":
		return statement{}, p.Errorf("%q is not supported: this shell runs MongoDB commands, not arbitrary JavaScript", id)
	default:
		return statement{}, p.Errorf("unknown command %q (try help)", id)
	}
	st.text = p.Slice(start, p.Pos())
	return st, nil
}

func parseChain(p *mql.Parser) ([]segment, error) {
	var segs []segment
	for {
		switch p.Peek() {
		case '.':
			p.Consume('.')
			name, ok := p.Ident()
			if !ok {
				return nil, p.Errorf("expected a name after '.'")
			}
			segs = append(segs, segment{name: name})
		case '[':
			p.Consume('[')
			name, ok, err := p.String()
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, p.Errorf("expected a quoted collection name inside [ ]")
			}
			if err := p.Expect(']'); err != nil {
				return nil, err
			}
			segs = append(segs, segment{name: name})
		case '(':
			if len(segs) == 0 || segs[len(segs)-1].call {
				return nil, p.Errorf("unexpected '('")
			}
			p.Consume('(')
			args, err := parseArgs(p)
			if err != nil {
				return nil, err
			}
			segs[len(segs)-1].call = true
			segs[len(segs)-1].args = args
		default:
			return segs, nil
		}
	}
}

func parseArgs(p *mql.Parser) ([]any, error) {
	var args []any
	for !p.Consume(')') {
		if id, ok := peekIdent(p); ok && (id == "function" || id == "async") {
			return nil, p.Errorf("JavaScript functions are not supported as arguments")
		}
		v, err := p.Value()
		if err != nil {
			return nil, err
		}
		args = append(args, v)
		if !p.Consume(',') {
			if err := p.Expect(')'); err != nil {
				return nil, err
			}
			break
		}
	}
	return args, nil
}

func peekIdent(p *mql.Parser) (string, bool) {
	p.SkipSpace()
	save := p.Pos()
	id, ok := p.Ident()
	p.Seek(save)
	return id, ok
}

// describe renders a chain for error messages, e.g. db.users.find().
func describe(segs []segment) string {
	var b strings.Builder
	b.WriteString("db")
	for _, s := range segs {
		fmt.Fprintf(&b, ".%s", s.name)
		if s.call {
			b.WriteString("()")
		}
	}
	return b.String()
}
