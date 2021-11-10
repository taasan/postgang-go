package ical

import (
	"fmt"
	"io"
	"log"
	"strings"
)

type ContentPrinter struct {
	writer            PrintWriter
	currentLineLength int
	err               error
	bytesWritten      int
	errorsAreFatal    bool
}

type PrintWriter interface {
	io.StringWriter
	WriteRune(r rune) (n int, err error)
}

func NewContentPrinter(wr PrintWriter, errorsAreFatal bool) *ContentPrinter {
	return &ContentPrinter{writer: wr, errorsAreFatal: errorsAreFatal}
}

func (p *ContentPrinter) printLn() {
	n, err := p.writer.WriteString("\r\n")
	p.bytesWritten += n
	p.err = err
	if err == nil {
		p.currentLineLength = 0
	} else if p.errorsAreFatal {
		log.Fatal(err)
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
		p.bytesWritten = bytesWritten
		if p.err != nil && p.errorsAreFatal {
			log.Fatal(p.err)
		}
		return p
	}
	for {
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
				if perror != nil {
					return doReturn()
				}
				p.currentLineLength = 1
			}
			if toPrint == "" {
				n, perror = p.writer.WriteRune(r)
			} else {
				n, perror = p.writer.WriteString(toPrint)
			}
			bytesWritten += bytesRead
			p.currentLineLength += n
		}
	}
}

func (p *ContentPrinter) printAttribute(a *Attribute) *ContentPrinter {
	p.print(a.Name, true).
		print("=", false).
		print(a.Value, true)
	return p
}

func (p *ContentPrinter) printField(f *icalField) *ContentPrinter {
	p.print(f.name, true)
	for _, a := range f.attributes {
		p.print(";", false).
			printAttribute(a)
	}
	p.print(":", false).
		print(f.value, true).
		printLn()
	return p
}

func (section *Section) Print(p *ContentPrinter) error {
	for _, field := range section.getFields() {
		p = p.printField(field)
		if p.err != nil {
			return p.err
		}
	}
	return p.err
}

func (section *Section) String() string {
	var sb = &strings.Builder{}
	section.Print(NewContentPrinter(sb, true))
	return sb.String()
}
