// Package requiredtag reports struct literals that leave out a required field.
//
// Go has no required fields. Every field is optional, and a field left out of
// a literal silently gets its zero value. requiredtag keeps that default and
// lets a struct opt in, field by field, with a struct tag. Which tags mark a
// field required is configured, so the tags a codebase already writes can do
// the job:
//
//	type CancelArgs struct {
//		TenantID   domain.TenantID   `lint:"required"`
//		TransferID domain.TransferID `lint:"required"`
//		Reason     string
//	}
//
//	CancelArgs{TenantID: t} // CancelArgs is missing required field TransferID
//
// A literal must set every required field by name. Positional literals are
// skipped, because the compiler already makes them list every field.
package requiredtag

import (
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"reflect"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Settings configures the analyzer. golangci-lint decodes it from the
// linter's settings block.
type Settings struct {
	// Tags lists the markers that make a field required, each written
	// key:option. A field is required when the value of its key tag, split on
	// commas, contains option. validate:required matches `validate:"required"`
	// and `validate:"required,email"`, but not `validate:"required_if=Kind x"`.
	Tags []string `json:"tags"`

	// AllowEmptyReturns accepts an empty literal returned next to a result
	// that says there is no value: a non-nil error, as in `return T{}, err`,
	// or false, as in `return T{}, false`. The caller must not use that value.
	AllowEmptyReturns bool `json:"allow-empty-returns"`
}

// New returns the analyzer configured by s.
func New(s Settings) (*analysis.Analyzer, error) {
	markers, err := parseMarkers(s.Tags)
	if err != nil {
		return nil, err
	}

	c := checker{markers: markers, allowEmptyReturns: s.AllowEmptyReturns}

	return &analysis.Analyzer{
		Name:     "requiredtag",
		Doc:      "reports struct literals that leave out a field marked required by a struct tag",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run:      c.run,
	}, nil
}

// marker is one configured tag. A field is required when the value of its key
// tag lists option.
type marker struct {
	key    string
	option string
}

func parseMarkers(tags []string) ([]marker, error) {
	if len(tags) == 0 {
		return nil, errors.New("requiredtag: tags is empty; list the tags that mark a field required, for example validate:required")
	}

	markers := make([]marker, 0, len(tags))

	for _, t := range tags {
		key, option, ok := strings.Cut(t, ":")
		if !ok || key == "" || option == "" {
			return nil, fmt.Errorf("requiredtag: tag %q must be written key:option, for example validate:required", t)
		}

		markers = append(markers, marker{key: key, option: option})
	}

	return markers, nil
}

type checker struct {
	markers           []marker
	allowEmptyReturns bool
}

func (c checker) run(pass *analysis.Pass) (any, error) {
	insp, _ := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	filter := []ast.Node{(*ast.CompositeLit)(nil)}

	insp.WithStack(filter, func(n ast.Node, push bool, stack []ast.Node) bool {
		if lit, ok := n.(*ast.CompositeLit); ok && push {
			c.check(pass, lit, stack)
		}

		return true
	})

	return nil, nil
}

// check reports the required fields that lit leaves out. stack holds lit's
// ancestors, with lit last.
func (c checker) check(pass *analysis.Pass, lit *ast.CompositeLit, stack []ast.Node) {
	typ := pass.TypesInfo.TypeOf(lit)
	if typ == nil || isPositional(lit) {
		return
	}

	st, ok := deref(typ).Underlying().(*types.Struct)
	if !ok {
		return
	}

	required := c.requiredFields(pass.Pkg, st)
	if len(required) == 0 {
		return
	}

	if len(lit.Elts) == 0 && c.allowEmptyReturns && returnedAsAbsent(pass, stack) {
		return
	}

	missing := missingFields(lit, required)
	if len(missing) == 0 {
		return
	}

	name := types.TypeString(deref(typ), func(p *types.Package) string {
		if p == pass.Pkg {
			return ""
		}

		return p.Name()
	})
	noun := "field"

	if len(missing) > 1 {
		noun = "fields"
	}

	pass.Reportf(lit.Pos(), "%s is missing required %s %s", name, noun, strings.Join(missing, ", "))
}

// requiredFields lists the fields of st that a marker makes required and that
// code in pkg can set. An unexported field of another package's struct can only
// be set by that package, so it is left to that package's own literals.
func (c checker) requiredFields(pkg *types.Package, st *types.Struct) []string {
	var names []string

	for i := range st.NumFields() {
		field := st.Field(i)
		if !field.Exported() && field.Pkg() != pkg {
			continue
		}

		if c.marks(reflect.StructTag(st.Tag(i))) {
			names = append(names, field.Name())
		}
	}

	return names
}

func (c checker) marks(tag reflect.StructTag) bool {
	for _, m := range c.markers {
		value, ok := tag.Lookup(m.key)
		if !ok {
			continue
		}

		for option := range strings.SplitSeq(value, ",") {
			if strings.TrimSpace(option) == m.option {
				return true
			}
		}
	}

	return false
}

func missingFields(lit *ast.CompositeLit, required []string) []string {
	set := make(map[string]bool, len(lit.Elts))

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		if key, ok := kv.Key.(*ast.Ident); ok {
			set[key.Name] = true
		}
	}

	var missing []string

	for _, name := range required {
		if !set[name] {
			missing = append(missing, name)
		}
	}

	return missing
}

func isPositional(lit *ast.CompositeLit) bool {
	if len(lit.Elts) == 0 {
		return false
	}

	_, keyed := lit.Elts[0].(*ast.KeyValueExpr)

	return !keyed
}

// returnedAsAbsent reports whether the literal on top of stack is returned
// next to a result that says there is no value: a non-nil error, as in
// `return T{}, err`, or false, as in `return T{}, false`.
func returnedAsAbsent(pass *analysis.Pass, stack []ast.Node) bool {
	parent := len(stack) - 2
	if parent >= 0 {
		if u, ok := stack[parent].(*ast.UnaryExpr); ok && u.Op == token.AND {
			parent--
		}
	}

	if parent < 0 {
		return false
	}

	ret, ok := stack[parent].(*ast.ReturnStmt)
	if !ok || len(ret.Results) < 2 {
		return false
	}

	for _, result := range ret.Results {
		tv := pass.TypesInfo.Types[result]

		switch {
		case tv.Type == nil || tv.IsNil():
			continue
		case tv.Value != nil && tv.Value.Kind() == constant.Bool && !constant.BoolVal(tv.Value):
			return true
		case types.Implements(tv.Type, errorType):
			return true
		}
	}

	return false
}

var errorType = types.Universe.Lookup("error").Type().Underlying().(*types.Interface) //nolint:forcetypeassert // error's underlying type is always an interface

// deref returns the type a pointer points to, or t itself. An elided literal
// inside a []*T has type *T.
func deref(t types.Type) types.Type {
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		return p.Elem()
	}

	return t
}
