// Package ai turns a natural-language request into a MongoDB query or
// aggregation proposal. Guarantees:
//
//   - Privacy: the model sees the collection name and a schema summary (field
//     paths, types, presence). Sample values are only sent when the user opts
//     in, and then only after masking. Connection strings, hosts, user names
//     and credentials are never part of the request.
//   - Validation: every proposal is parsed with the shared mql parser,
//     checked for write stages/operators, and validated by the server with an
//     explain at queryPlanner verbosity (planned, never executed).
//   - No auto-execution: this package only proposes. Reads are loaded into
//     an editor; writes need an explicit, confirmed request (see Service.Execute).
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/aggregate"
	"mango3s/internal/apperr"
	"mango3s/internal/mql"
	"mango3s/internal/query"
	"mango3s/internal/schema"
)

// Kinds of proposals.
const (
	KindFind       = "find"
	KindAggregate  = "aggregate"
	KindUpdateMany = "updateMany"
	KindDeleteMany = "deleteMany"
)

// Request asks for a proposal.
type Request struct {
	ConnID         string `json:"connId"`
	DB             string `json:"db"`
	Coll           string `json:"coll"`
	Prompt         string `json:"prompt"`
	Mode           string `json:"mode"`           // auto | find | aggregate
	IncludeSamples bool   `json:"includeSamples"` // send masked sample documents
	AllowWrites    bool   `json:"allowWrites"`    // permit updateMany/deleteMany/$out/$merge proposals
}

// FieldSummary is what the model learns about one field.
type FieldSummary struct {
	Path     string   `json:"path"`
	Types    []string `json:"types"`
	Presence float64  `json:"presence"`
}

// Context is exactly what is sent to the provider (shown to the user).
type Context struct {
	Collection string         `json:"collection"`
	Fields     []FieldSummary `json:"fields"`
	Samples    []string       `json:"samples"`
}

// Check is one validation step.
type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// Proposal is a validated suggestion.
type Proposal struct {
	ID                   string            `json:"id"`
	Kind                 string            `json:"kind"`
	DB                   string            `json:"db"`
	Coll                 string            `json:"coll"`
	Filter               string            `json:"filter"`
	Projection           string            `json:"projection"`
	Sort                 string            `json:"sort"`
	Limit                int64             `json:"limit"`
	Pipeline             []aggregate.Stage `json:"pipeline"`
	Update               string            `json:"update"`
	Explanation          string            `json:"explanation"`
	Warnings             []string          `json:"warnings"`
	IsWrite              bool              `json:"isWrite"`
	RequiresConfirmation bool              `json:"requiresConfirmation"`
	Valid                bool              `json:"valid"`
	Validation           []Check           `json:"validation"`
	Shell                string            `json:"shell"`
	Sent                 Context           `json:"sent"`
	Model                string            `json:"model"`
	InputTokens          int64             `json:"inputTokens"`
	OutputTokens         int64             `json:"outputTokens"`
	DurationMs           int64             `json:"durationMs"`
	ConnID               string            `json:"-"`
}

// Usage is token accounting from the provider.
type Usage struct {
	InputTokens, OutputTokens int64
	Model                     string
}

// Provider generates the raw JSON answer. Implementations must not log the prompt.
type Provider interface {
	Generate(ctx context.Context, system, user string, schema map[string]any) (string, Usage, error)
}

// answer is the structured output the model must produce. Values are strings
// in MongoDB shell syntax so they go through the same parser as user input.
type answer struct {
	Kind       string `json:"kind"`
	Filter     string `json:"filter"`
	Projection string `json:"projection"`
	Sort       string `json:"sort"`
	Limit      int64  `json:"limit"`
	Pipeline   []struct {
		Operator string `json:"operator"`
		Body     string `json:"body"`
	} `json:"pipeline"`
	Update      string   `json:"update"`
	Explanation string   `json:"explanation"`
	Warnings    []string `json:"warnings"`
}

// AnswerSchema is the JSON schema for structured outputs.
func AnswerSchema() map[string]any {
	str := map[string]any{"type": "string"}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"kind", "filter", "projection", "sort", "limit", "pipeline", "update", "explanation", "warnings"},
		"properties": map[string]any{
			"kind":       map[string]any{"type": "string", "enum": []string{KindFind, KindAggregate, KindUpdateMany, KindDeleteMany}},
			"filter":     str,
			"projection": str,
			"sort":       str,
			"limit":      map[string]any{"type": "integer"},
			"pipeline": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object", "additionalProperties": false, "required": []string{"operator", "body"},
					"properties": map[string]any{"operator": str, "body": str},
				},
			},
			"update":      str,
			"explanation": str,
			"warnings":    map[string]any{"type": "array", "items": str},
		},
	}
}

// SystemPrompt instructs the model. It contains no connection details.
const SystemPrompt = `You translate requests about a MongoDB collection into one query.

Answer with the JSON object described by the schema:
- kind "find": put the query in filter/projection/sort/limit; leave pipeline empty and update "".
- kind "aggregate": put stages in pipeline as {operator: "$match", body: "{ ... }"}; leave filter/projection/sort "" and limit 0.
- kind "updateMany" / "deleteMany": only when the user clearly asks to change or delete data; filter selects documents, update holds the update document or pipeline.
Write every document as MongoDB shell syntax (e.g. { status: "A", createdAt: { $gte: ISODate("2024-01-01T00:00:00Z") } }). Use "" for unused document fields and 0 for no limit.
Use only fields listed in the schema summary; if the request needs a field that is not listed, say so in warnings.
Prefer read-only queries. Explain briefly what the query does in explanation.`

// BuildContext summarizes a schema report for the model. Samples, when
// given, must already be masked.
func BuildContext(coll string, rep schema.Report, samples []bson.D) Context {
	c := Context{Collection: coll, Fields: []FieldSummary{}, Samples: []string{}}
	for _, f := range rep.Fields {
		fs := FieldSummary{Path: f.Path, Presence: f.Presence}
		for _, t := range f.Types {
			fs.Types = append(fs.Types, t.Type)
		}
		c.Fields = append(c.Fields, fs)
		if len(c.Fields) >= 300 {
			break
		}
	}
	for _, s := range samples {
		c.Samples = append(c.Samples, mql.Compact(s))
	}
	return c
}

// UserMessage renders the request and context.
func UserMessage(req Request, c Context) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Collection: %s\n\nFields (path: types, %% of documents):\n", c.Collection)
	for _, f := range c.Fields {
		fmt.Fprintf(&b, "- %s: %s, %.0f%%\n", f.Path, strings.Join(f.Types, "|"), f.Presence)
	}
	if len(c.Samples) > 0 {
		b.WriteString("\nSample documents (sensitive values masked):\n")
		for _, s := range c.Samples {
			b.WriteString(s + "\n")
		}
	}
	switch req.Mode {
	case KindFind:
		b.WriteString("\nAnswer with kind \"find\".\n")
	case KindAggregate:
		b.WriteString("\nAnswer with kind \"aggregate\".\n")
	}
	if !req.AllowWrites {
		b.WriteString("\nOnly read queries are allowed: do not use updateMany, deleteMany, $out or $merge.\n")
	}
	fmt.Fprintf(&b, "\nRequest: %s", req.Prompt)
	return b.String()
}

var writeStages = map[string]bool{"$out": true, "$merge": true}

// Validate parses and checks a raw answer. It does not contact the server.
func Validate(raw string, req Request) (*Proposal, error) {
	var a answer
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return nil, apperr.New(apperr.CodeMongo, "The AI response was not valid JSON: %v", err)
	}
	p := &Proposal{Kind: a.Kind, DB: req.DB, Coll: req.Coll, Explanation: a.Explanation, Warnings: a.Warnings,
		Filter: strings.TrimSpace(a.Filter), Projection: strings.TrimSpace(a.Projection), Sort: strings.TrimSpace(a.Sort),
		Limit: a.Limit, Update: strings.TrimSpace(a.Update), Pipeline: []aggregate.Stage{}, Validation: []Check{}}
	if p.Warnings == nil {
		p.Warnings = []string{}
	}
	add := func(name string, ok bool, format string, args ...any) {
		p.Validation = append(p.Validation, Check{Name: name, OK: ok, Detail: fmt.Sprintf(format, args...)})
	}
	syntaxOK := true
	parse := func(label, text string) {
		if text == "" {
			return
		}
		if _, err := mql.ParseValue(text); err != nil {
			syntaxOK = false
			add("Syntax: "+label, false, "%v", err)
		}
	}
	switch a.Kind {
	case KindFind:
		parse("filter", p.Filter)
		parse("projection", p.Projection)
		parse("sort", p.Sort)
		if syntaxOK {
			if _, err := query.Parse(query.Request{Filter: p.Filter, Projection: p.Projection, Sort: p.Sort, Limit: max(p.Limit, 0)}); err != nil {
				syntaxOK = false
				add("Structure", false, "%v", apperr.From(err).Message)
			}
		}
		if p.Limit < 0 {
			p.Limit = 0
		}
	case KindAggregate:
		for _, st := range a.Pipeline {
			p.Pipeline = append(p.Pipeline, aggregate.Stage{Operator: strings.TrimSpace(st.Operator), Body: strings.TrimSpace(st.Body), Enabled: true})
			if writeStages[strings.TrimSpace(st.Operator)] {
				p.IsWrite = true
			}
		}
		if len(p.Pipeline) == 0 {
			syntaxOK = false
			add("Structure", false, "the pipeline is empty")
		} else if _, err := aggregate.Build(p.Pipeline, -1); err != nil {
			syntaxOK = false
			add("Syntax: pipeline", false, "%v", apperr.From(err).Message)
		}
	case KindUpdateMany, KindDeleteMany:
		p.IsWrite = true
		parse("filter", p.Filter)
		if a.Kind == KindUpdateMany {
			if p.Update == "" {
				syntaxOK = false
				add("Structure", false, "updateMany needs an update")
			}
			parse("update", p.Update)
			if syntaxOK {
				if err := checkUpdate(p.Update); err != nil {
					syntaxOK = false
					add("Structure", false, "%v", err)
				}
			}
		}
		if f, err := mql.ParseDocument(p.Filter); err == nil && len(f) == 0 {
			p.Warnings = append(p.Warnings, "The filter is empty: this affects every document in the collection.")
		}
	default:
		return nil, apperr.New(apperr.CodeMongo, "The AI returned an unknown kind %q.", a.Kind)
	}
	if syntaxOK {
		add("Syntax", true, "parsed with the shell-syntax parser")
	}
	p.RequiresConfirmation = p.IsWrite
	if p.IsWrite && !req.AllowWrites {
		add("Read-only policy", false, "the proposal writes data, but write proposals are not allowed for this request")
	} else if p.IsWrite {
		add("Write", true, "writes data; it will only run after you confirm, with a backup of affected documents")
	}
	p.Shell = shell(p)
	p.Valid = true
	for _, c := range p.Validation {
		p.Valid = p.Valid && c.OK
	}
	return p, nil
}

func checkUpdate(text string) error {
	v, err := mql.ParseValue(text)
	if err != nil {
		return err
	}
	switch u := v.(type) {
	case bson.D:
		if len(u) == 0 || !strings.HasPrefix(u[0].Key, "$") {
			return fmt.Errorf("the update must use operators such as $set (whole-document replacement is not allowed)")
		}
	case bson.A:
	default:
		return fmt.Errorf("the update must be a document or pipeline")
	}
	return nil
}

func orEmpty(s string) string {
	if s == "" {
		return "{}"
	}
	return s
}

func shell(p *Proposal) string {
	coll := query.CollectionRef(p.Coll)
	switch p.Kind {
	case KindFind:
		s := fmt.Sprintf("%s.find(%s", coll, orEmpty(p.Filter))
		if p.Projection != "" {
			s += ", " + p.Projection
		}
		s += ")"
		if p.Sort != "" {
			s += ".sort(" + p.Sort + ")"
		}
		if p.Limit > 0 {
			s += fmt.Sprintf(".limit(%d)", p.Limit)
		}
		return s
	case KindAggregate:
		parts := make([]string, len(p.Pipeline))
		for i, st := range p.Pipeline {
			parts[i] = fmt.Sprintf("  { %s: %s }", st.Operator, st.Body)
		}
		return coll + ".aggregate([\n" + strings.Join(parts, ",\n") + "\n])"
	case KindUpdateMany:
		return fmt.Sprintf("%s.updateMany(%s, %s)", coll, orEmpty(p.Filter), p.Update)
	case KindDeleteMany:
		return fmt.Sprintf("%s.deleteMany(%s)", coll, orEmpty(p.Filter))
	}
	return ""
}

// timeSince is a seam for tests.
var timeSince = time.Since
