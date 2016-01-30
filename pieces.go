package sparse

import "fmt"

type Kind interface {
	kind()
}

type pieceKind int

func (pieceKind) kind() {}

const (
	KindField pieceKind = 1 + iota
	KindComment
	KindNodeEnter
	KindNodeLeave
)

type Piece interface {
	piece()
	Kind() Kind
	String() string
}

type Field struct{ Key, Value string }

func (Field) piece() {}
func (f Field) String() string {
	if f.Value == "" {
		return keyEscaper.Replace(f.Key) + "!"
	}
	return keyEscaper.Replace(f.Key) + " " + valueEscaper.Replace(f.Value)
}
func (f Field) Kind() Kind       { return KindField }
func (f Field) GoString() string { return fmt.Sprintf("%T(%q: %q)", f, f.Key, f.Value) }

type Comment string

func (Comment) piece()             {}
func (c Comment) String() string   { return "#" + string(c) }
func (c Comment) Kind() Kind       { return KindComment }
func (c Comment) GoString() string { return fmt.Sprintf("%T(%q)", c, string(c)) }

type NodeEnter string

func (NodeEnter) piece()             {}
func (s NodeEnter) String() string   { return string(s) + "{" }
func (s NodeEnter) Kind() Kind       { return KindNodeEnter }
func (s NodeEnter) GoString() string { return fmt.Sprintf("%T(%q)", s, string(s)) }

type NodeLeave int

func (NodeLeave) piece()             {}
func (NodeLeave) String() string     { return "}" }
func (NodeLeave) Kind() Kind         { return KindNodeLeave }
func (l NodeLeave) GoString() string { return fmt.Sprintf("%T(%d)", l, l) }
