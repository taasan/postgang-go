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

func (p *ContentPrinter) printLn() *ContentPrinter {
	n, err := p.writer.WriteString("\r\n")
	p.bytesWritten += n
	p.err = err
	if err == nil {
		p.currentLineLength = 0
	} else if p.errorsAreFatal {
		log.Fatal(err)
	}
	return p
}

func (p *ContentPrinter) print(value string) *ContentPrinter {
	if p.err != nil {
		return p
	}
	const CRLFS = "\r\n "
	bytesWritten := 0
	reader := strings.NewReader(strings.ReplaceAll(value, "\n", "\\n"))
	var n int
	var perror error
	doReturn := func() *ContentPrinter {
		p.err = perror
		p.bytesWritten = bytesWritten
		if p.err != nil && p.errorsAreFatal {
			log.Panic(p.err)
		}
		return p
	}
	for {
		if r, bytesRead, readErr := reader.ReadRune(); readErr != nil {
			return doReturn()
		} else {
			if bytesRead+p.currentLineLength > maxLineLen {
				n, perror = p.writer.WriteString(CRLFS)
				bytesWritten += n
				if perror != nil {
					return doReturn()
				}
				p.currentLineLength = 1
			}
			n, perror = p.writer.WriteRune(r)
			bytesWritten += bytesRead
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

func (p *ContentPrinter) printAttribute(a *Attribute) *ContentPrinter {
	p.print(escape(a.Name))
	p.print("=")
	p.print(escape(a.Value))
	return p
}

func (p *ContentPrinter) printField(f *icalField) *ContentPrinter {
	p.print(escape(f.name))
	for _, a := range f.attributes {
		p.print(";").
			printAttribute(a)
	}
	p.print(":").
		print(escape(f.value)).
		printLn()
	return p
}

func (section *Section) Print(p *ContentPrinter) (int, error) {
	for _, field := range section.getFields() {
		var before = p.bytesWritten
		p = p.printField(field)
		if p.err != nil {
			return p.bytesWritten - before, p.err
		}
	}
	return p.bytesWritten, p.err
}
