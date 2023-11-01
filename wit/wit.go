package wit

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ydnar/wasm-tools-go/internal/codec"
)

// Node is the interface implemented by the WIT ([WebAssembly Interface Type])
// types in this package.
//
// [WebAssembly Interface Type]: https://component-model.bytecodealliance.org/design/wit.html
type Node interface {
	WIT(ctx Node, name string) string
}

type _node struct{}

func (_node) WIT(ctx Node, name string) string { return "/* TODO(" + name + ") */" }

func indent(s string) string {
	const ws = "    "
	return strings.TrimSuffix(ws+strings.ReplaceAll(s, "\n", "\n"+ws), ws)
}

// unwrap unwraps a multiline string into a single line, if:
// 1. its length is <= 50 chars
// 2. its line count is <= 5
// This is used for single-line [Record], [Flags], [Variant], and [Enum] declarations.
func unwrap(s string) string {
	const chars = 50
	const lines = 5
	if len(s) > chars || strings.Count(s, "\n") > lines {
		return s
	}
	var b strings.Builder
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			b.WriteRune(' ')
		}
		b.WriteString(strings.Trim(line, " \t\r\n"))
	}
	return b.String()
}

// WIT returns the [WIT] text format for [Resolve] r. Note that the return value could
// represent multiple files, so may not be precisely valid WIT text.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (r *Resolve) WIT(_ Node, _ string) string {
	var b strings.Builder
	for i, p := range r.Packages {
		if i > 0 {
			b.WriteRune('\n')
			b.WriteRune('\n')
		}
		b.WriteString(p.WIT(r, ""))
	}
	return b.String()
}

// WIT returns the [WIT] text format for [World] w.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (w *World) WIT(ctx Node, name string) string {
	if name == "" {
		name = w.Name
	}
	var b strings.Builder
	// TODO: docs
	b.WriteString("world ")
	b.WriteString(escape(name)) // TODO: compare to w.Name?
	b.WriteString(" {")
	n := 0
	for _, name := range codec.SortedKeys(w.Imports) {
		if f, ok := w.Imports[name].(*Function); ok {
			if _, ok := f.Kind.(*Freestanding); !ok {
				continue
			}
		}
		if n == 0 {
			b.WriteRune('\n')
		}
		b.WriteString(indent(w.itemWIT("import", name, w.Imports[name])))
		b.WriteRune('\n')
		n++
	}
	for _, name := range codec.SortedKeys(w.Exports) {
		if n == 0 {
			b.WriteRune('\n')
		}
		b.WriteString(indent(w.itemWIT("export", name, w.Exports[name])))
		b.WriteRune('\n')
		n++
	}
	b.WriteRune('}')
	return b.String()
}

func (w *World) itemWIT(motion, name string, v WorldItem) string {
	switch v := v.(type) {
	case *Interface, *Function:
		return motion + " " + v.WIT(w, name) // TODO: handle resource methods?
	case *TypeDef:
		return v.WIT(w, name) // no motion, in Imports only
	}
	panic("BUG: unknown WorldItem")
}

// WIT returns the [WIT] text format for [Interface] i.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (i *Interface) WIT(ctx Node, name string) string {
	if i.Name != nil && name == "" {
		name = *i.Name
	}

	var b strings.Builder

	// TODO: docs

	switch ctx := ctx.(type) {
	case *Package:
		b.WriteString("interface ")
		b.WriteString(escape(name))
		b.WriteRune(' ')
	case *World:
		rname := relativeName(i, ctx.Package)
		if rname != "" {
			return escape(rname) + ";"
		}

		// Otherwise, this is an inline interface decl.
		b.WriteString(escape(name))
		b.WriteString(": interface ")
	}

	b.WriteRune('{')
	n := 0
	for _, name := range codec.SortedKeys(i.TypeDefs) {
		if n == 0 {
			b.WriteRune('\n')
		}
		b.WriteString(indent(i.TypeDefs[name].WIT(i, name)))
		b.WriteRune('\n')
		n++
	}
	for _, name := range codec.SortedKeys(i.Functions) {
		f := i.Functions[name]
		if _, ok := f.Kind.(*Freestanding); !ok {
			continue
		}
		if n == 0 {
			b.WriteRune('\n')
		}
		b.WriteString(indent(f.WIT(i, name)))
		b.WriteRune('\n')
		n++
	}
	b.WriteRune('}')
	return b.String()
}

// WIT returns the [WIT] text format for [TypeDef] t.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (t *TypeDef) WIT(ctx Node, name string) string {
	if t.Name != nil && name == "" {
		name = *t.Name
	}
	switch ctx := ctx.(type) {
	// If context is another TypeDef, then this is an imported type.
	case *TypeDef:
		// Emit an type alias if same Owner.
		if t.Owner == ctx.Owner && t.Name != nil {
			return "type " + escape(name) + " = " + escape(*t.Name)
		}
		ownerName := relativeName(t.Owner, ctx.Package())
		if t.Name != nil && *t.Name != name {
			return fmt.Sprintf("use %s.{%s as %s};", ownerName, escape(*t.Name), escape(name))
		}
		return fmt.Sprintf("use %s.{%s};", ownerName, escape(name))

	case *World, *Interface:
		var b strings.Builder
		b.WriteString(t.Kind.WIT(t, name))
		constructor := t.Constructor()
		methods := t.Methods()
		statics := t.StaticFunctions()
		if constructor != nil || len(methods) > 0 || len(statics) > 0 {
			b.WriteString(" {\n")
			if constructor != nil {
				b.WriteString(indent(constructor.WIT(t, "constructor")))
				b.WriteRune('\n')
			}
			slices.SortFunc(methods, functionCompare)
			for _, f := range methods {
				b.WriteString(indent(f.WIT(t, "")))
				b.WriteRune('\n')
			}
			slices.SortFunc(statics, functionCompare)
			for _, f := range statics {
				b.WriteString(indent(f.WIT(t, "")))
				b.WriteRune('\n')
			}
			b.WriteRune('}')
		}
		s := b.String()
		if s[len(s)-1] != '}' && s[len(s)-1] != ';' {
			b.WriteRune(';')
		}
		return b.String()
	}
	if name != "" {
		return escape(name)
	}
	return t.Kind.WIT(ctx, name)
}

func functionCompare(a, b *Function) int {
	return strings.Compare(a.Name, b.Name)
}

func escape(name string) string {
	if keywords[name] {
		return "%" + name
	}
	return name
}

var keywords = map[string]bool{
	"enum":      true,
	"export":    true,
	"flags":     true,
	"func":      true,
	"import":    true,
	"include":   true,
	"interface": true,
	"package":   true,
	"record":    true,
	"resource":  true,
	"result":    true,
	"static":    true,
	"type":      true,
	"variant":   true,
	"world":     true,
}

func relativeName(o TypeOwner, p *Package) string {
	var op *Package
	var name string
	switch o := o.(type) {
	case *Interface:
		if o.Name == nil {
			return ""
		}
		op = o.Package
		name = *o.Name

	case *World:
		op = o.Package
		name = o.Name
	}
	if op == p {
		return name
	}
	if op == nil {
		return ""
	}
	return op.Name.String() + "/" + name
}

// WIT returns the [WIT] text format for [Record] r.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (r *Record) WIT(ctx Node, name string) string {
	var b strings.Builder
	b.WriteString("record ")
	b.WriteString(escape(name))
	b.WriteString(" {")
	if len(r.Fields) > 0 {
		b.WriteRune('\n')
		for i := range r.Fields {
			if i > 0 {
				b.WriteString(",\n")
			}
			b.WriteString(indent(r.Fields[i].WIT(ctx, "")))
		}
		b.WriteRune('\n')
	}
	b.WriteRune('}')
	return unwrap(b.String())
}

// WIT returns the [WIT] text format for [Field] f.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (f *Field) WIT(ctx Node, name string) string {
	// TODO: docs
	return escape(f.Name) + ": " + f.Type.WIT(f, "")
}

// WIT returns the [WIT] text format for [Resource] r.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (r *Resource) WIT(ctx Node, name string) string {
	var b strings.Builder
	b.WriteString("resource ")
	b.WriteString(escape(name))
	return b.String()
}

// WIT returns the [WIT] text format for [OwnedHandle] h.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (h *OwnedHandle) WIT(ctx Node, name string) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("type ")
		b.WriteString(escape(name))
		b.WriteString(" = ")
	}
	b.WriteString("own<")
	b.WriteString(h.Type.WIT(h, ""))
	b.WriteRune('>')
	return b.String()
}

// WIT returns the [WIT] text format for [BorrowedHandle] h.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (h *BorrowedHandle) WIT(ctx Node, name string) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("type ")
		b.WriteString(escape(name))
		b.WriteString(" = ")
	}
	b.WriteString("borrow<")
	b.WriteString(h.Type.WIT(h, ""))
	b.WriteRune('>')
	return b.String()
}

// WIT returns the [WIT] text format for [Flags] f.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (f *Flags) WIT(ctx Node, name string) string {
	var b strings.Builder
	b.WriteString("flags ")
	b.WriteString(escape(name))
	b.WriteString(" {")
	if len(f.Flags) > 0 {
		for i := range f.Flags {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(f.Flags[i].WIT(f, ""))
		}
	}
	b.WriteRune('}')
	return unwrap(b.String())
}

// WIT returns the [WIT] text format for [Flag] f.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (f *Flag) WIT(_ Node, _ string) string {
	// TODO: docs
	return escape(f.Name)
}

// WIT returns the [WIT] text format for [Tuple] t.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (t *Tuple) WIT(ctx Node, _ string) string {
	var b strings.Builder
	b.WriteString("tuple<")
	for i := range t.Types {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(t.Types[i].WIT(t, ""))
	}
	b.WriteString(">")
	return b.String()
}

// WIT returns the [WIT] text format for [Variant] v.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (v *Variant) WIT(_ Node, name string) string {
	var b strings.Builder
	b.WriteString("variant ")
	b.WriteString(escape(name))
	b.WriteString(" {")
	if len(v.Cases) > 0 {
		b.WriteRune('\n')
		for i := range v.Cases {
			if i > 0 {
				b.WriteString(",\n")
			}
			b.WriteString(indent(v.Cases[i].WIT(v, "")))
		}
		b.WriteRune('\n')
	}
	b.WriteRune('}')
	return unwrap(b.String())
}

// WIT returns the [WIT] text format for [Case] c.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (c *Case) WIT(_ Node, _ string) string {
	// TODO: docs
	var b strings.Builder
	b.WriteString(escape(c.Name))
	if c.Type != nil {
		b.WriteRune('(')
		b.WriteString(c.Type.WIT(c, ""))
		b.WriteRune(')')
	}
	return b.String()
}

// WIT returns the [WIT] text format for [Enum] e.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (e *Enum) WIT(_ Node, name string) string {
	var b strings.Builder
	b.WriteString("enum ")
	b.WriteString(escape(name))
	b.WriteString(" {")
	if len(e.Cases) > 0 {
		b.WriteRune('\n')
		for i := range e.Cases {
			if i > 0 {
				b.WriteString(",\n")
			}
			b.WriteString(indent(e.Cases[i].WIT(e, "")))
		}
		b.WriteRune('\n')
	}
	b.WriteRune('}')
	return unwrap(b.String())
}

// WIT returns the [WIT] text format for [EnumCase] c.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (c *EnumCase) WIT(_ Node, _ string) string {
	// TODO: docs
	return escape(c.Name)
}

// WIT returns the [WIT] text format for [Option] o.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (o *Option) WIT(_ Node, name string) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("type ")
		b.WriteString(escape(name))
		b.WriteString(" = ")
	}
	b.WriteString("option<")
	b.WriteString(o.Type.WIT(o, ""))
	b.WriteRune('>')
	return b.String()
}

// WIT returns the [WIT] text format for [Result] r.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (r *Result) WIT(_ Node, name string) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("type ")
		b.WriteString(escape(name))
		b.WriteString(" = ")
	}
	b.WriteString("result<")
	if r.OK != nil {
		b.WriteString(r.OK.WIT(r, ""))
	} else {
		b.WriteRune('_')
	}
	if r.Err != nil {
		b.WriteString(", ")
		b.WriteString(r.Err.WIT(r, ""))
	}
	b.WriteRune('>')
	return b.String()
}

// WIT returns the [WIT] text format for [List] l.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (l *List) WIT(_ Node, name string) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("type ")
		b.WriteString(escape(name))
		b.WriteString(" = ")
	}
	b.WriteString("list<")
	b.WriteString(l.Type.WIT(l, ""))
	b.WriteRune('>')
	return b.String()
}

// WIT returns the [WIT] text format for [Future] f.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (f *Future) WIT(_ Node, name string) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("type ")
		b.WriteString(escape(name))
		b.WriteString(" = ")
	}
	b.WriteString("future")
	if f.Type != nil {
		b.WriteRune('<')
		b.WriteString(f.Type.WIT(f, ""))
		b.WriteRune('>')
	}
	return b.String()
}

// WIT returns the [WIT] text format for [Stream] s.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (s *Stream) WIT(_ Node, name string) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("type ")
		b.WriteString(escape(name))
		b.WriteString(" = ")
	}
	b.WriteString("stream<")
	if s.Element != nil {
		b.WriteString(s.Element.WIT(s, ""))
		b.WriteString(", ")
	} else {
		b.WriteString("_, ")
	}
	if s.End != nil {
		b.WriteString(s.End.WIT(s, ""))
	} else {
		b.WriteRune('_')
	}
	b.WriteRune('>')
	return b.String()
}

// WIT returns the [WIT] text format for this [TypeDefKind].
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (p _primitive[T]) WIT(_ Node, name string) string {
	if name != "" {
		return "type " + name + " = " + p.String()
	}
	return p.String()
}

// WIT returns the [WIT] text format for [Function] f.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (f *Function) WIT(_ Node, name string) string {
	if name == "" {
		name = f.Name
		if _, after, found := strings.Cut(name, "."); found {
			name = after
		}
	}
	// TODO: docs
	var b strings.Builder
	b.WriteString(escape(name))
	b.WriteString(": func(")
	b.WriteString(paramsWIT(f.Params))
	b.WriteRune(')')
	if len(f.Results) > 0 {
		b.WriteString(" -> ")
		b.WriteString(paramsWIT(f.Results))
	}
	b.WriteRune(';')
	return b.String()
}

func paramsWIT(params []Param) string {
	var b strings.Builder
	for i, param := range params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(param.WIT(nil, ""))
	}
	return b.String()
}

// WIT returns the [WIT] text format of [Param] p.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (p *Param) WIT(_ Node, _ string) string {
	if p.Name == "" {
		return p.Type.WIT(p, "")
	}
	return p.Name + ": " + p.Type.WIT(p, "")
}

// WIT returns the [WIT] text format of [Package] p.
//
// [WIT]: https://github.com/WebAssembly/component-model/blob/main/design/mvp/WIT.md
func (p *Package) WIT(ctx Node, _ string) string {
	// TODO: docs
	var b strings.Builder
	b.WriteString("package ")
	b.WriteString(p.Name.String())
	b.WriteString(";\n")
	if len(p.Interfaces) > 0 {
		b.WriteRune('\n')
		for i, name := range codec.SortedKeys(p.Interfaces) {
			if i > 0 {
				b.WriteRune('\n')
			}
			b.WriteString(p.Interfaces[name].WIT(p, name))
			b.WriteRune('\n')
		}
	}
	if len(p.Worlds) > 0 {
		b.WriteRune('\n')
		for i, name := range codec.SortedKeys(p.Worlds) {
			if i > 0 {
				b.WriteRune('\n')
			}
			b.WriteString(p.Worlds[name].WIT(p, name))
			b.WriteRune('\n')
		}
	}
	return b.String()
}
