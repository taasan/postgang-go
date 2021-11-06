package ical

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

const maxLineLen = 75

type Field struct {
	Name       string
	Attributes []Attribute
	Value      string
}

type Attribute struct {
	Name  string
	Value string
}

func escapeChar(c rune) string {
	if c == '\\' || c == ';' || c == ',' {
		return fmt.Sprintf("\\%c", c)
	}
	if c == '\n' {
		return "\\n"
	}
	return fmt.Sprintf("%c", c)
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

func splitLine(line string) []string {
	splits := []string{}
	var l, r int
	for l, r = 0, maxLineLen; r < len(line); l, r = r, r+maxLineLen {
		for !utf8.RuneStart(line[r]) {
			r--
		}
		splits = append(splits, line[l:r])
	}
	splits = append(splits, line[l:])
	return splits
}

func (f Field) String() string {
	var sb strings.Builder
	sb.WriteString(escape(f.Name))
	for _, a := range f.Attributes {
		sb.WriteByte(';')
		sb.WriteString(escape(a.Name))
		sb.WriteByte('=')
		sb.WriteString(escape(a.Value))
	}
	sb.WriteByte(':')
	sb.WriteString(escape(f.Value))
	if len(sb.String()) > maxLineLen {
		lines := splitLine(sb.String())
		sb.Reset()
		sb.WriteString(lines[0])
		sb.WriteString("\r\n")
		for _, l := range lines[1:] {
			sb.WriteByte(' ')
			sb.WriteString(l)
			sb.WriteString("\r\n")
		}
	} else {
		sb.WriteString("\r\n")
	}
	return sb.String()
}

func WriteIcal(wr io.StringWriter, fields ...Field) (int, error) {
	var written = 0
	var err error
	for _, x := range fields {
		var n, err = wr.WriteString(x.String())
		written += n
		if err != nil {
			return written, err
		}
	}
	return written, err
}

func dateAttribute() Attribute {
	return Attribute{
		Name:  "VALUE",
		Value: "DATE",
	}
}

func New(name string, value interface{}, attributes ...Attribute) Field {
	return Field{
		Name:       name,
		Value:      fmt.Sprint(value),
		Attributes: attributes,
	}
}

func dateField(name string, value time.Time) Field {
	return New(name, value.Format("20060102"), dateAttribute())
}

func DtStart(value time.Time) Field {
	return dateField("DTSTART", value)
}

func DtEnd(value time.Time) Field {
	return dateField("DTEND", value)
}

func DtStamp(value time.Time) Field {
	return New("DTSTAMP", value.Format("20060102T150405Z"))
}

func Section(name string, fields []Field, attributes ...Attribute) []Field {
	buf := []Field{New("BEGIN", name, attributes...)}
	buf = append(buf, fields...)
	return append(buf, New("END", name))
}
