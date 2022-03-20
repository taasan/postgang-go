package ical

import (
	"fmt"
	"io"
	"log"
	"strings"
)

const maxLineLen = 75

type ContentPrinter struct {
	writer            PrintWriter
	currentLineLength int
	err               error
}

type PrintWriter interface {
	io.StringWriter
	WriteRune(r rune) (n int, err error)
}

func (p *ContentPrinter) Error() error {
	return p.err
}

func NewContentPrinter(wr PrintWriter) *ContentPrinter {
	return &ContentPrinter{
		writer:            wr,
		currentLineLength: 0,
		err:               nil,
	}
}

func (p *ContentPrinter) printLn() {
	_, err := p.writer.WriteString("\r\n")
	p.err = err
	if err == nil {
		p.currentLineLength = 0
	}
}

func (p *ContentPrinter) print(value string, escape bool) *ContentPrinter {
	if p.err != nil || value == "" {
		return p
	}
	const CRLFS = "\r\n "
	reader := strings.NewReader(value)
	var n int
	var perror error
	doReturn := func() *ContentPrinter {
		p.err = perror
		return p
	}
	for perror == nil {
		if r, _, readErr := reader.ReadRune(); readErr != nil {
			return doReturn()
		} else {
			var toPrint string
			if escape && (r == '\\' || r == ';' || r == ',') {
				toPrint = fmt.Sprintf("\\%c", r)
			} else if r == '\n' {
				toPrint = "\\n"
			} else {
				toPrint = string(r)
			}
			if len(toPrint)+p.currentLineLength > maxLineLen {
				_, perror = p.writer.WriteString(CRLFS)
				p.currentLineLength = 1
			}
			n, perror = p.writer.WriteString(toPrint)
			p.currentLineLength += n
		}
	}
	return doReturn()
}

func (p *ContentPrinter) printAttribute(a *Attribute) *ContentPrinter {
	p.print(a.Name, false).
		print("=", false).
		print(a.Value, true)
	return p
}

func (p *ContentPrinter) printField(f *icalField) *ContentPrinter {
	p.print(f.name, false)
	for _, a := range f.attributes {
		p.print(";", false).
			printAttribute(a)
	}
	p.print(":", false).
		print(f.value, true).
		printLn()
	return p
}

func (p *ContentPrinter) Print(content icalContent) *ContentPrinter {
	for _, field := range content.fields() {
		p = p.printField(field)
		if p.err != nil {
			return p
		}
	}
	return p
}

func (section *Section) String() string {
	var sb = &strings.Builder{}
	p := NewContentPrinter(sb).Print(section)
	if p.err != nil {
		log.Panic(p.err)
	}
	return sb.String()
}
