package sparse

import (
	"io"
	"unicode"
)

type Reader interface {
	io.Reader
	io.RuneReader
}

type ASCIIReader struct{ io.Reader }

func (r ASCIIReader) ReadByte() (byte, error) {
	var buf [1]byte
	_, err := r.Read(buf[:])
	return buf[0], err
}

func (r ASCIIReader) ReadRune() (rune, int, error) {
	b, err := r.ReadByte()
	if err != nil {
		return rune(b), 1, err
	} else if b&0x8 != 0 {
		return unicode.ReplacementChar, 1, nil
	}
	return rune(b), 1, err
}

func (r ASCIIReader) ReadBytes(delim byte) ([]byte, error) {
	return readUntil(r.Reader, delim)
}
