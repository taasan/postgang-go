package ical

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

type ContentPrinter struct {
	Writer            *bufio.Writer
	currentLineLength int
	err               error
	bytesWritten      int
}

func (p *ContentPrinter) printLine(value fmt.Stringer) (int, error) {
	if p.err != nil {
		return 0, p.err
	}
	if p.currentLineLength != 0 {
		p.err = errors.New("internal error: p.currentLineLength != 0")
		return p.bytesWritten, p.err
	}
	const CRLF = "\r\n"
	const CRLFS = "\r\n "
	bytesWritten := 0
	reader := strings.NewReader(strings.ReplaceAll(value.String(), "\n", "\\n"))
	for {
		if c, n, err := reader.ReadRune(); err != nil {
			if err == io.EOF {
				n, _ := p.Writer.WriteString(CRLF)
				bytesWritten += n
				p.currentLineLength = 0
				p.bytesWritten += bytesWritten
				return bytesWritten, nil
			}
			p.err = err
			p.bytesWritten += bytesWritten
			return bytesWritten, err
		} else {
			if n+p.currentLineLength > maxLineLen {
				n, _ = p.Writer.WriteString(CRLFS)
				bytesWritten += n
				p.currentLineLength = 1
			}
			n, _ = p.Writer.WriteRune(c)
			bytesWritten += n
			p.currentLineLength += n
		}
	}
}

func escapeChar(c rune) string {
	if c == '\\' || c == ';' || c == ',' {
		return fmt.Sprintf("\\%c", c)
	}
	return string(c)
}

func escape(s string) string {
	var sb strings.Builder
	for _, c := range s {
		sb.WriteString(escapeChar(c))
	}
	return sb.String()
}

func (f *Attribute) String() string {
	var sb strings.Builder
	sb.WriteString(escape(f.Name))
	sb.WriteRune('=')
	sb.WriteString(escape(f.Value))
	return sb.String()
}

func (f *icalField) String() string {
	var sb strings.Builder
	sb.WriteString(escape(f.name))
	for _, a := range f.attributes {
		sb.WriteByte(';')
		sb.WriteString(escape(a.Name))
		sb.WriteByte('=')
		sb.WriteString(escape(a.Value))
	}
	sb.WriteByte(':')
	sb.WriteString(escape(f.value))

	return sb.String()
}

func Print(p *ContentPrinter, section *Section) (int, error) {
	for _, field := range section.getFields() {
		n, err := p.printLine(field)
		if err != nil {
			return n, err
		}
	}
	return p.bytesWritten, p.err
}