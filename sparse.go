package sdf

import (
	"bufio"
	"bytes"
	"io"
)

type Handler interface {
	Comment(comment string) error
	Enter(key string) error
	Leave() error
	Field(key, value string) error
}

type Parser struct {
	ReadComments       bool
	CompressWhitespace bool
	TrimWhitespace     bool

	lastKey string
}

var DefaultParser = Parser{
	ReadComments:       false,
	CompressWhitespace: true,
	TrimWhitespace:     true,
}

func Parse(r io.Reader, h Handler) error {
	parser := DefaultParser
	return parser.Parse(r, h)
}

type readFn func(*bufio.Reader, Handler) (readFn, error)

func (p *Parser) comment(comment string, next readFn) readFn {
	return func(r *bufio.Reader, h Handler) (readFn, error) {
		var err error
		if h != nil {
			err = h.Comment(comment)
		}
		return next, err
	}
}

func (p *Parser) readComment(next readFn) readFn {
	return func(r *bufio.Reader, h Handler) (readFn, error) {
		comment, err := r.ReadBytes('\n')
		if err == nil {
			// chomp line ending
			comment = comment[:len(comment)-1]
		}

		if err != nil && err != io.EOF {
			// If err isn't nil post-comment read, then nullify the
			// next read function
			next = nil
		}

		if p.ReadComments {
			return p.comment(string(comment), next), err
		} else {
			return next, err
		}
	}
}

func (p *Parser) readKey(r *bufio.Reader, h Handler) (readFn, error) {
	c, _, err := r.ReadRune()
	for (c == ' ' || c == '\t' || c == '\n' || c == '\r') && err == nil {
		c, _, err = r.ReadRune()
	}

	if err != nil && err != io.EOF {
		return nil, err
	}

	if c == '}' {
		return p.leave, err
	} else if c == '#' {
		return p.readComment(p.readKey), err
	}

	var escape bool
	var last rune
	var key bytes.Buffer
	for !(!escape && (c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '#')) && err == nil {
		if !escape && c == '\\' {
			escape = true
			goto skipWrite
		} else if escape {
			if c == 't' {
				c = '\t'
			} else if c == 'n' {
				c = '\n'
			} else if c == 'r' {
				c = '\r'
			} else if c == 'b' {
				c = '\b'
			} else if c == 'f' {
				c = '\f'
			} else if c == '0' {
				c = rune(0)
			} else if c == 'v' {
				c = '\v'
			}
			escape = false
			goto skipCompressCheck
		} else if c == '\r' {
			goto skipWrite
		}

		if p.CompressWhitespace && (c == '\t' || c == ' ') && last == c {
			goto skipWrite
		}

	skipCompressCheck:
		last = c
		key.WriteRune(c)
	skipWrite:
		c, _, err = r.ReadRune()
	}

	if key.Len() == 0 && err != nil {
		return nil, err
	} else if err == nil || err == io.EOF {
		p.lastKey = key.String()
	}

	next := p.readValue
	if err == io.EOF {
		next = p.field(p.lastKey, "", nil)
	} else if c == '#' {
		next = p.readComment(next)
	}

	return next, err
}

func (p *Parser) enter(key string) readFn {
	return func(r *bufio.Reader, h Handler) (readFn, error) {
		var err error
		if h != nil {
			err = h.Enter(key)
		}
		return p.readKey, err
	}
}

func (p *Parser) leave(r *bufio.Reader, h Handler) (readFn, error) {
	var err error
	if h != nil {
		err = h.Leave()
	}
	return p.readKey, err
}

func (p *Parser) field(key, value string, next readFn) readFn {
	return func(r *bufio.Reader, h Handler) (readFn, error) {
		var err error
		if h != nil {
			err = h.Field(key, value)
		}
		return next, err
	}
}

// readValue attempts to read a value from the given bufio Reader and returns
// the next read function or an error.
func (p *Parser) readValue(r *bufio.Reader, h Handler) (readFn, error) {
	c, _, err := r.ReadRune()
	for (c == ' ' || c == '\t' || c == '\n' || c == '\r') && err == nil {
		c, _, err = r.ReadRune()
	}

	if err != nil && err != io.EOF {
		return nil, err
	}

	if c == '{' {
		return p.enter(p.lastKey), err
	} else if c == '#' {
		return p.readComment(p.readKey), h.Field(p.lastKey, "")
	}

	var escape bool
	var last rune
	var value bytes.Buffer
	for !(!escape && (c == '\n')) && err == nil {
		if !escape {
			if c == '\\' {
				escape = true
				goto skipWrite
			} else if c == '#' {
				break
			}
		} else if escape {
			if c == 't' {
				c = '\t'
			} else if c == 'n' {
				c = '\n'
			} else if c == 'r' {
				c = '\r'
			} else if c == 'b' {
				c = '\b'
			} else if c == 'f' {
				c = '\f'
			} else if c == '0' {
				c = rune(0)
			} else if c == 'v' {
				c = '\v'
			}
			escape = false
			goto skipCompressCheck
		} else if c == '\r' {
			goto skipWrite
		}

	skipCompressCheck:
		if p.CompressWhitespace && (c == '\t' || c == ' ') && last == c {
			goto skipWrite
		}

		last = c
		value.WriteRune(c)
	skipWrite:
		c, _, err = r.ReadRune()
	}

	next := p.readKey
	if err == io.EOF {
		next = nil
	} else if c == '#' {
		next = p.readComment(next)
	}

	var valueStr string
	if value.Len() > 0 {
		if p.TrimWhitespace {
			valueStr = string(bytes.TrimRight(value.Bytes(), " \n\t\r"))
		} else {
			valueStr = value.String()
		}
	}

	return p.field(p.lastKey, valueStr, next), err
}

// PArse attempts to read a Sparse data file from the given reader. If it
// succeeds, no error is returned. Otherwise, if either the Handler or another
// error occurs, that error is returned.
func (p *Parser) Parse(rd io.Reader, h Handler) error {
	r := bufio.NewReader(rd)
	reader := p.readKey
	var err error

	for reader != nil && err == nil {
		reader, err = reader(r, h)

		if err == io.EOF && reader != nil {
			reader(r, h)
		}
	}

	p.lastKey = ""

	if err == io.EOF {
		err = nil
	}

	return err
}
