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
	errorsAreFatal    bool
}

type PrintWriter interface {
	io.StringWriter
	WriteRune(r rune) (n int, err error)
}

func (p *ContentPrinter) Error() error {
	return p.err
}

func NewContentPrinter(wr PrintWriter, errorsAreFatal bool) *ContentPrinter {
	return &ContentPrinter{writer: wr, errorsAreFatal: errorsAreFatal}
}

func (p *ContentPrinter) printLn() {
	_, err := p.writer.WriteString("\r\n")
	p.err = err
	if err == nil {
		p.currentLineLength = 0
	} else if p.errorsAreFatal {
		log.Panic(err)
	}
}

func (p *ContentPrinter) print(value string, escape bool) *ContentPrinter {
	if p.err != nil || value == "" {
		return p
	}
	const CRLFS = "\r\n "
	bytesWritten := 0
	reader := strings.NewReader(value)
	var n int
	var perror error
	doReturn := func() *ContentPrinter {
		p.err = perror
		if p.err != nil && p.errorsAreFatal {
			log.Panic(p.err)
		}
		return p
	}
	for perror == nil {
		if r, bytesRead, readErr := reader.ReadRune(); readErr != nil {
			return doReturn()
		} else {
			var toPrint = ""
			if escape && (r == '\\' || r == ';' || r == ',') {
				toPrint = fmt.Sprintf("\\%c", r)
				bytesRead = len(toPrint)
			} else if r == '\n' {
				toPrint = "\\n"
				bytesRead = len(toPrint)
			}
			if bytesRead+p.currentLineLength > maxLineLen {
				n, perror = p.writer.WriteString(CRLFS)
				bytesWritten += n
				p.currentLineLength = 1
			}
			if toPrint == "" {
				n, perror = p.writer.WriteRune(r)
			} else {
				n, perror = p.writer.WriteString(toPrint)
			}
			bytesWritten += n
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
	NewContentPrinter(sb, true).Print(section)
	return sb.String()
}
