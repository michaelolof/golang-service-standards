package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var (
	methods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}
	scalars = map[string]bool{"string": true, "integer": true, "number": true, "boolean": true}
)

// checkOpenAPI reads each committed document in docs/openapi, which gofi writes
// and task generate:check keeps current (HTTP-7). Every parameter and field of
// every operation has a description, and every scalar an example (HTTP-4). The
// document shows what the code can't: the fields of the domain types and
// generic envelopes a schema uses. An operation's tag, description and
// operationId are checked in its handler, by the gofihandlers analyzer (HTTP-5).
func checkOpenAPI(m *module) ([]problem, error) {
	files, err := filepath.Glob(filepath.Join(m.root, "docs", "openapi", "*.json"))
	if err != nil {
		return nil, err
	}

	var problems []problem

	for _, file := range files {
		found, err := checkDocument(m.rel(file), file)
		if err != nil {
			return nil, err
		}

		problems = append(problems, found...)
	}

	return problems, nil
}

type document struct {
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components struct {
		Schemas map[string]*schema `json:"schemas"`
	} `json:"components"`
}

type operation struct {
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
	OperationID string   `json:"operationId"`
	Parameters  []struct {
		Name        string          `json:"name"`
		In          string          `json:"in"`
		Description string          `json:"description"`
		Example     json.RawMessage `json:"example"`
		Examples    json.RawMessage `json:"examples"`
		Schema      *schema         `json:"schema"`
	} `json:"parameters"`
	RequestBody *struct {
		Content map[string]struct {
			Schema *schema `json:"schema"`
		} `json:"content"`
	} `json:"requestBody"`
	Responses map[string]struct {
		Content map[string]struct {
			Schema *schema `json:"schema"`
		} `json:"content"`
	} `json:"responses"`
}

type schema struct {
	Ref                  string             `json:"$ref"`
	Type                 json.RawMessage    `json:"type"`
	Description          string             `json:"description"`
	Example              json.RawMessage    `json:"example"`
	Examples             json.RawMessage    `json:"examples"`
	Properties           map[string]*schema `json:"properties"`
	Items                *schema            `json:"items"`
	AdditionalProperties json.RawMessage    `json:"additionalProperties"`
	AllOf                []*schema          `json:"allOf"`
	AnyOf                []*schema          `json:"anyOf"`
	OneOf                []*schema          `json:"oneOf"`
}

// gaps collects the fields of one operation or schema that lack a description
// or an example.
type gaps struct {
	description, example []string
}

func checkDocument(rel, path string) ([]problem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}

	text := string(data)

	var problems []problem

	for _, route := range sortedKeys(doc.Paths) {
		for _, method := range methods {
			raw, ok := doc.Paths[route][method]
			if !ok {
				continue
			}

			var op operation
			if err := json.Unmarshal(raw, &op); err != nil {
				return nil, fmt.Errorf("%s: %s %s: %w", rel, method, route, err)
			}

			where := strings.ToUpper(method) + " " + route
			line := lineOf(text, route)

			for _, msg := range checkOperation(where, op) {
				problems = append(problems, problem{path: rel, line: line, msg: msg})
			}
		}
	}

	for _, name := range sortedKeys(doc.Components.Schemas) {
		var g gaps

		g.walk("", doc.Components.Schemas[name])

		for _, msg := range g.messages("schema " + name) {
			problems = append(problems, problem{path: rel, line: lineOf(text, name), msg: msg})
		}
	}

	return problems, nil
}

func checkOperation(where string, op operation) []string {
	var g gaps

	for _, p := range op.Parameters {
		name := p.In + " " + p.Name

		if p.Description == "" && (p.Schema == nil || p.Schema.Description == "") {
			g.description = append(g.description, name)
		}

		if !hasExample(p.Example, p.Examples) && (p.Schema == nil || !hasExample(p.Schema.Example, p.Schema.Examples)) {
			g.example = append(g.example, name)
		}
	}

	if op.RequestBody != nil {
		for _, media := range sortedKeys(op.RequestBody.Content) {
			g.walk("body", op.RequestBody.Content[media].Schema)
		}
	}

	for _, status := range sortedKeys(op.Responses) {
		for _, media := range sortedKeys(op.Responses[status].Content) {
			g.walk(status, op.Responses[status].Content[media].Schema)
		}
	}

	return g.messages(where)
}

// walk records each property under s, at path, that lacks a description, or,
// for a scalar, an example. A $ref is checked where it's declared.
func (g *gaps) walk(path string, s *schema) {
	if s == nil || s.Ref != "" {
		return
	}

	for _, name := range sortedKeys(s.Properties) {
		prop := s.Properties[name]
		if prop == nil || prop.Ref != "" {
			continue
		}

		field := strings.TrimPrefix(path+"."+name, ".")

		if prop.Description == "" {
			g.description = append(g.description, field)
		}

		if scalars[typeOf(prop)] && !hasExample(prop.Example, prop.Examples) {
			g.example = append(g.example, field)
		}

		g.walk(field, prop)
	}

	g.walk(path+"[]", s.Items)

	var additional schema
	if json.Unmarshal(s.AdditionalProperties, &additional) == nil {
		g.walk(path+"{}", &additional)
	}

	for _, sub := range slices.Concat(s.AllOf, s.AnyOf, s.OneOf) {
		g.walk(path, sub)
	}
}

func (g *gaps) messages(where string) []string {
	var msgs []string

	if len(g.description) > 0 {
		msgs = append(msgs, fmt.Sprintf("%s: %d fields have no description: %s (HTTP-4)", where, len(g.description), summary(g.description)))
	}

	if len(g.example) > 0 {
		msgs = append(msgs, fmt.Sprintf("%s: %d scalar fields have no example: %s (HTTP-4)", where, len(g.example), summary(g.example)))
	}

	return msgs
}

// typeOf returns a schema's type. OpenAPI 3.1 allows a list, such as
// ["string", "null"], whose first non-null entry counts.
func typeOf(s *schema) string {
	var one string
	if json.Unmarshal(s.Type, &one) == nil {
		return one
	}

	var many []string
	if json.Unmarshal(s.Type, &many) == nil {
		for _, t := range many {
			if t != "null" {
				return t
			}
		}
	}

	return ""
}

func hasExample(example, examples json.RawMessage) bool {
	return len(example) > 0 || len(examples) > 0
}

// summary lists the first few fields, so a long list stays readable.
func summary(fields []string) string {
	const shown = 8
	if len(fields) <= shown {
		return strings.Join(fields, ", ")
	}

	return strings.Join(fields[:shown], ", ") + fmt.Sprintf(" and %d more", len(fields)-shown)
}

// lineOf returns the line of the first "key": in text, or 0.
func lineOf(text, key string) int {
	quoted, _ := json.Marshal(key)

	i := strings.Index(text, string(quoted)+":")
	if i < 0 {
		return 0
	}

	return strings.Count(text[:i], "\n") + 1
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	return keys
}
