package ical

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

type Failer struct {
	err          error
	failAfter    int
	bytesWritten int
}

func (wr *Failer) WriteString(s string) (int, error) {
	if wr.err != nil {
		return 0, wr.err
	}
	var n int
	if wr.bytesWritten > wr.failAfter {
		wr.err = fmt.Errorf("Should fail after %d, have written %d", wr.failAfter, wr.bytesWritten)
		n = 0
	} else {
		n = len(s)
	}
	wr.bytesWritten += n
	return n, wr.err
}
func (wr *Failer) WriteRune(c rune) (int, error) {
	return wr.WriteString(string(c))
}

func TestContentPrinterPrintWithErrSet(t *testing.T) {
	var sb = &strings.Builder{}
	p := NewContentPrinter(sb, false)
	err := errors.New("Test")
	p.err = err
	p = p.print("A", true)
	if p.err != err {
		t.Fail()
	}
	if sb.String() != "" {
		t.Fail()
	}
}

// ☣️
func TestContentPrinterPrintLongLine(t *testing.T) {
	var sb = &strings.Builder{}
	p := NewContentPrinter(sb, false)
	p.print(fmt.Sprintf("%0*d", maxLineLen+3, 0), true)
	expected := fmt.Sprintf("%0*d\r\n 000", maxLineLen, 0)
	got := sb.String()
	if got != expected {
		t.Logf("\n%s\n!=\n%s\nLengths %d != %d", got, expected, len(got), len(expected))
		t.Fail()
	}
}
func TestContentPrinterPrintLongLineEmoji(t *testing.T) {
	var sb = &strings.Builder{}
	p := NewContentPrinter(sb, false)
	p.print("A☣️☣️☣️☣️☣️☣️☣️☣️☣️☣️☣️☣️☣️☣️☣️☣️☣️☣️", true)
	expected := "A☣️☣️☣️☣️☣️☣️☣️☣️☣️☣️☣️☣️\r\n ☣️☣️☣️☣️☣️☣️"
	got := sb.String()
	if got != expected {
		t.Logf("\n%s\n!=\n%s\nLengths %d != %d", got, expected, len(got), len(expected))
		t.Fail()
	}
}

func TestContentPrinterPrintLn(t *testing.T) {
	var sb = &strings.Builder{}
	p := NewContentPrinter(sb, false)
	p.printLn()
	expected := "\r\n"
	got := sb.String()
	if got != expected {
		t.Logf("\n%s\n!=\n%s\nLengths %d != %d", got, expected, len(got), len(expected))
		t.Fail()
	}
}

func TestContentPrinterPrintEscaped(t *testing.T) {
	var sb = &strings.Builder{}
	p := NewContentPrinter(sb, false)
	p.print(",;\\\n", true)
	expected := `\,\;\\\n`
	got := sb.String()
	if got != expected {
		t.Logf("\n%s\n!=\n%s\nLengths %d != %d", got, expected, len(got), len(expected))
		t.Fail()
	}
}

func TestContentPrinterPrintEscapedSemicolon(t *testing.T) {
	var sb = &strings.Builder{}
	p := NewContentPrinter(sb, false)
	p.print(";", false)
	expected := ";"
	got := sb.String()
	if got != expected {
		t.Logf("\n%s\n!=\n%s\nLengths %d != %d", got, expected, len(got), len(expected))
		t.Fail()
	}
}

func TestContentPrinterError(t *testing.T) {
	p := NewContentPrinter(&Failer{}, false)
	p.print("asdas", false)
	if p.err == nil {
		t.FailNow()
	}
	if p.err != p.Error() {
		t.Log(p.err)
		t.Log(p.Error())
		t.Fail()
	}
}

func TestContentPrinterLongLineError(t *testing.T) {
	failer := &Failer{failAfter: maxLineLen + 10}
	p := NewContentPrinter(failer, false)
	p.print(fmt.Sprintf("%0*d", failer.failAfter+5, 0), true)
	if p.err == nil {
		t.FailNow()
	}
	if p.err != p.Error() {
		t.Log(p.err)
		t.Log(p.Error())
		t.Fail()
	}
}

func icalFields() []*icalField {
	return []*icalField{
		field("VERSION", "2.0"),
		field("CALSCALE", "GREGORIAN"),
		field("METHOD", "PUBLISH"),
		field("VEV", "Value", dateAttribute()),
	}
}

func sectionFixture() *Section {
	return &Section{
		name: "VCAL",
		content: &Fields{
			Fields: icalFields(),
		},
	}
}

func TestContentPrintWithError(t *testing.T) {
	failer := &Failer{}
	p := NewContentPrinter(failer, false)
	p.Print(sectionFixture())
	if p.Error() == nil {
		t.Fail()
	}
}

func TestContentString(t *testing.T) {
	got := sectionFixture().String()
	expected := strings.Join([]string{
		"BEGIN:VCAL",
		"VERSION:2.0",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		"VEV;VALUE=DATE:Value",
		"END:VCAL",
		"",
	}, "\r\n")
	if got != expected {
		t.Logf("\n%s\n!=\n%s\nLengths %d != %d", got, expected, len(got), len(expected))
		t.Fail()
	}
}
