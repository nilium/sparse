// Package sparse implements a sparse definition parser.
//
// Sparse is effectively an attribute language parser, where the language resembles Quake 3 style shaders. For example:
//
//      textures/base/wall_arc_01 {
//              { # unit
//                      map textures/base/wall_arc_01.tga
//              }
//              {
//                      map textures/base/wall_arc_01.glow.tga
//                      blend add
//              }
//
//              no-collision!
//              depth lte
//              alpha always
//              grid
//                      1     1     1 \
//                      1     1     1 \
//                      1     1     1
//      }
//
// Which would yield the following Pieces (either via a Parser or calling Parse):
//
//      NodeEnter("textures/base/wall_arc_01")
//      NodeEnter(" unit")
//      Comment(" unit") // If ReadComments(true)
//      Field{"map", "textures/base/wall_arc_01.tga"}
//      NodeLeave(2)
//      NodeEnter()
//      Field{"map", "textures/base/wall_arc_01.glow.tga"}
//      Field{"blend", "add"}
//      NodeLeave(2)
//      Field{"no-collision", ""}
//      Field{"depth", "lte"}
//      Field{"alpha", "always"}
//      Field{"grid", "1 1 1\n1 1 1\n1 1 1"
//      NodeLeave(1)
//
package sparse

// TODO(nilium): Need to write up-to-date / correct documentation since this is a renovation of an older package.

import (
	"bytes"
	"errors"
	"io"
	"unicode"
)

type Parser struct {
	readComments           bool
	keepSeqWhitespace      bool
	keepTrailingWhitespace bool

	depth int
	next  parser
	buf   bytes.Buffer
}

func (p *Parser) Reset(configs ...Configuration) {
	p.buf.Reset()
	*p = Parser{buf: p.buf}
	for _, cfg := range configs {
		cfg.apply(p)
	}
}

type bytesReader interface {
	ReadBytes(delim byte) ([]byte, error)
}

func readByte(r io.Reader) (byte, error) {
	if r, ok := r.(io.ByteReader); ok {
		return r.ReadByte()
	}

	var rd [1]byte
	_, err := r.Read(rd[:])
	return rd[0], err
}

func readUntil(r io.Reader, delim byte) (seq []byte, err error) {
	if r, ok := r.(bytesReader); ok {
		return r.ReadBytes(delim)
	}

	buf := make([]byte, 0, 16)
	for err == nil {
		var b byte
		if b, err = readByte(r); err == nil {
			buf = append(buf, b)
		}
		if b == delim {
			break
		}
	}
	return buf, err
}

func NewParser(configs ...Configuration) *Parser {
	p := new(Parser)
	p.Reset(configs...)
	return p
}

func Parse(r Reader, configs ...Configuration) (pieces []Piece, err error) {
	var p Parser
	p.Reset(configs...)

	for err == nil {
		var piece Piece
		piece, err = p.Read(r)
		if err == nil {
			pieces = append(pieces, piece)
		}
	}

	if err == io.EOF {
		err = nil
	}

	return pieces, err
}

type parser interface {
	read(Reader) (parser, Piece, error)
}

type readFn func(Reader) (parser, Piece, error)

func (fn readFn) read(r Reader) (parser, Piece, error) { return fn(r) }

func (p *Parser) comment(comment string, next parser) parser {
	return readFn(func(r Reader) (parser, Piece, error) {
		return next, Comment(comment), nil
	})
}

func (p *Parser) readComment(next parser) parser {
	return readFn(func(r Reader) (parser, Piece, error) {
		comment, err := readUntil(r, '\n')
		if err == nil {
			// chomp line ending
			comment = comment[:len(comment)-1]
		}

		if err != nil && err != io.EOF {
			// If err isn't nil post-comment read, then nullify the
			// next read function
			next = errReader{err}
		}

		var piece Piece
		if p.readComments {
			piece = Comment(string(comment))
		}

		return next, piece, err
	})
}

func (p *Parser) readKey(r Reader) (parser, Piece, error) {
	c, _, err := r.ReadRune()
	for (c == ' ' || c == '\t' || c == '\n' || c == '\r') && err == nil {
		c, _, err = r.ReadRune()
	}

	if err != nil && err != io.EOF {
		return eofReader, nil, err
	}

	if c == '}' {
		return p.leave()
	} else if c == '{' {
		return p.enter("")
	} else if c == '#' {
		return p.readComment(readFn(p.readKey)), nil, nil
	}

	var escape bool
	var last rune
	for !(!escape && (c == ' ' || c == '!' || c == ';' || c == '\t' || c == '\n' || c == '\r' || c == '#')) && err == nil {
		if c == '\r' {
			goto skipWrite
		}

		if !escape && c == '\\' {
			escape = true
			goto skipWrite
		} else if escape {
			if c == '\n' {
				if !p.keepSeqWhitespace {
					chompBuffer(&p.buf)
				}
			} else {
				switch c {
				case 't':
					c = '\t'
				case 'n':
					c = '\n'
				case 'r':
					c = '\r'
				case 'b':
					c = '\b'
				case 'f':
					c = '\f'
				case '0':
					c = rune(0)
				case 'v':
					c = '\v'
				}
			}
			escape = false
			goto skipCompressCheck
		} else if c == '\r' {
			goto skipWrite
		}

		if !p.keepSeqWhitespace && unicode.IsSpace(last) && unicode.IsSpace(c) {
			goto skipWrite
		}

	skipCompressCheck:
		last = c
		p.buf.WriteRune(c)
	skipWrite:
		c, _, err = r.ReadRune()
	}

	key := p.buf.String()
	p.buf.Reset()
	if len(key) == 0 && err != nil {
		if err == io.EOF {
			return eofReader, nil, err
		}

		return errReader{err}, nil, err
	}

	var next parser
	var piece Piece
	if err == io.EOF {
		next = eofReader
		piece = Field{key, ""}
	} else if c == '#' {
		next = p.readComment(next)
		piece = Field{key, ""}
	} else if c == '!' || c == ';' {
		next = readFn(p.readKey)
		piece = Field{key, ""}
	} else {
		next, piece, err = p.readValue(r, key)
	}

	return next, piece, err
}

func (p *Parser) enter(key string) (parser, Piece, error) {
	out := NodeEnter(key)
	p.depth++
	return readFn(p.readKey), out, nil
}

var ErrUnexpectedNodeLeave = errors.New("sparse: unexpected end of node")

func (p *Parser) leave() (parser, Piece, error) {
	if p.depth == 0 {
		return errReader{ErrUnexpectedNodeLeave}, nil, ErrUnexpectedNodeLeave
	}
	out := NodeLeave(p.depth)
	p.depth--
	return readFn(p.readKey), out, nil
}

func (p *Parser) field(key, value string, next parser) parser {
	return readFn(func(r Reader) (parser, Piece, error) {
		return next, Field{key, value}, nil
	})
}

func chompBuffer(b *bytes.Buffer) {
	n := b.Len()
	bs := b.Bytes()
	for ; n > 0; n-- {
		c := rune(bs[n-1])
		if c == '\n' || !unicode.IsSpace(rune(bs[n-1])) {
			break
		}
	}
	if b.Len() != n {
		b.Truncate(n)
	}
}

// readValue attempts to read a value from the given Reader and returns
// the next read function or an error.
func (p *Parser) readValue(r Reader, key string) (parser, Piece, error) {
	c, _, err := r.ReadRune()
	for (c == ' ' || c == '\t' || c == '\n' || c == '\r') && err == nil {
		c, _, err = r.ReadRune()
	}

	if err != nil && err != io.EOF {
		return errReader{err}, nil, err
	}

	if c == '{' {
		return p.enter(key)
	} else if c == '#' {
		return p.readComment(readFn(p.readKey)), Field{key, ""}, nil
	}

	defer p.buf.Reset()
	var escape bool
	var last rune
	for !(!escape && (c == '\n' || c == ';' || c == '#')) && err == nil {
		if c == '\r' {
			// Ignore entirely
			goto skipWrite
		}

		if !escape {
			if c == '\\' {
				escape = true
				goto skipWrite
			} else if c == '#' {
				break
			}
		} else if escape {
			if c == '\n' {
				if !p.keepSeqWhitespace {
					chompBuffer(&p.buf)
				}
			} else {
				switch c {
				case 't':
					c = '\t'
				case 'n':
					c = '\n'
				case 'r':
					c = '\r'
				case 'b':
					c = '\b'
				case 'f':
					c = '\f'
				case '0':
					c = rune(0)
				case 'v':
					c = '\v'
				}
			}
			escape = false
			goto skipCompressCheck
		} else if c == '\r' {
			goto skipWrite
		}

		if !p.keepSeqWhitespace && unicode.IsSpace(last) && unicode.IsSpace(c) {
			goto skipWrite
		}

	skipCompressCheck:
		last = c
		p.buf.WriteRune(c)
	skipWrite:
		c, _, err = r.ReadRune()
	}

	var next parser = readFn(p.readKey)
	if err == io.EOF {
		next = eofReader
	} else if c == '#' {
		next = p.readComment(next)
	}

	var valueStr string
	if p.buf.Len() > 0 {
		if !p.keepTrailingWhitespace {
			valueStr = string(bytes.TrimRight(p.buf.Bytes(), " \n\t\r"))
		} else {
			valueStr = p.buf.String()
		}
	}

	return next, Field{key, valueStr}, err
}

type eofReaderImpl struct{}

func (r eofReaderImpl) read(Reader) (parser, Piece, error) { return r, nil, io.EOF }

var eofReader eofReaderImpl

type errReader struct{ err error }

func (r errReader) read(Reader) (parser, Piece, error) {
	return r, nil, r.err
}

func (p *Parser) Read(r Reader) (piece Piece, err error) {
	if p.next == nil {
		p.next = readFn(p.readKey)
	}
	for p.next != nil && piece == nil && err == nil {
		p.next, piece, err = p.next.read(r)
	}

	return piece, err
}
