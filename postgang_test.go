package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
	"time"
)

// TestHelloName calls greetings.Hello with a name, checking
// for a valid return value.
func TestFromWeekdayName(t *testing.T) {
	for k, v := range weekdays {
		if k != weekdayNames[v] {
			t.Fatalf("%s => %s", k, v)
		}
	}
}

func TestFromWeekday(t *testing.T) {
	for k, v := range weekdayNames {
		if k != weekdays[v] {
			t.Fatalf("%s => %s", k, v)
		}
	}
}

func now() *time.Time {
	return calendarTFixture().dates[0].time
}

func prodID() string {
	return fmt.Sprintf("-//Aasan//Aasan Go Postgang %s@%s//EN", postalCode(), version)
}

func postalCode() *postalCodeT {
	postalCode, _ := toPostalCode("6666")
	return postalCode
}

//go:embed test/fixture*
var fixtures embed.FS

func dataFixture(t *testing.T) *postenResponseT {
	bs := readFixture("test/fixture.json", t)
	var data postenResponseT
	if err := json.NewDecoder(bytes.NewReader(bs)).Decode(&data); err != nil {
		return nil
	}
	return &data
}

func readFixture(name string, t *testing.T) []byte {
	bs, err := fixtures.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return bs
}

func TestReadData(t *testing.T) {
	bs := readFixture("test/fixture.json", t)
	p, _, _ := readData(now(), bytes.NewReader(bs))
	expected := dataFixture(t)
	if !reflect.DeepEqual(p, expected) {
		t.Fatalf("\n%+v\n\n!=\n\n%+v", p, expected)
	}
}

func addDay(t *time.Time, days int) *time.Time {
	n := t.AddDate(0, 0, days)
	return &n
}

func calendarTFixture() *calendarT {
	now := time.Date(2021, 12, 28, 0, 0, 0, 0, time.UTC)
	dates := []*CivilTime{
		{time: &now},
		{time: addDay(&now, 1)},
		{time: addDay(&now, 2)},
		{time: addDay(&now, 3)},
		{time: addDay(&now, 4)},
		{time: addDay(&now, 5)},
		{time: addDay(&now, 6)},
	}
	return &calendarT{
		now:      dates[0].time,
		prodID:   prodID(),
		dates:    dates,
		hostname: "test",
		code:     postalCode(),
	}
}

func TestToCalendarT(t *testing.T) {
	resp, now := dataFixture(t), now()
	cal := calendarTFixture()
	calendar := toCalendarT(now, resp, cal.hostname, postalCode())
	expectedCalendar := cal
	if !reflect.DeepEqual(calendar, expectedCalendar) {
		t.Fatalf("\n%+v\n\n!=\n\n%+v", calendar, expectedCalendar)
	}
}

func TestPrint(t *testing.T) {
	cal := toVCalendar(calendarTFixture())
	res := cal.String()
	fixtureName := "test/fixture.ics"
	icsFixture := readFixture(fixtureName, t)
	if res != string(icsFixture) {
		tmp, err := os.CreateTemp("", "postgang-*.ics")
		if err != nil {
			t.Fatal(err)
		}
		defer tmp.Close()
		if _, err := tmp.WriteString(res); err != nil {
			t.Fatal(err)
		}
		t.Fatalf("ICS mismatch, see\ndiff -u %s %s", fixtureName, tmp.Name())
	}
}

func commandLine() *flag.FlagSet {
	return flag.NewFlagSet("Test", flag.ContinueOnError)
}

func TestParseArgsCode(t *testing.T) {
	got, err := parseArgs(commandLine(), []string{"--code=" + postalCode().code})
	if err != nil {
		t.Fatal(err)
	}
	expected := commandLineArgs{code: postalCode()}
	if !reflect.DeepEqual(got.code, expected.code) {
		t.Fatalf("%s != %s", got.code, expected.code)
	}
}

func TestParseArgsInvalidCode(t *testing.T) {
	_, err := parseArgs(commandLine(), []string{"--code=99999"})
	if err == nil {
		t.Fatal("Expected error")
	}
}

func TestParseArgsInvalidDate(t *testing.T) {
	_, err := parseArgs(commandLine(), []string{"--date=20-a-n"})
	if err == nil {
		t.Fatal("Expected error")
	}
}

func TestParseArgsVersion(t *testing.T) {
	got, err := parseArgs(commandLine(), []string{"--version"})
	if err != nil {
		t.Fatal(err)
	}
	expected := commandLineArgs{version: true}
	if !reflect.DeepEqual(got.code, expected.code) {
		t.Fatalf("%s != %s", got.code, expected.code)
	}
	if got.code != nil {
		t.Fatalf("I didn't expect a code! %s", got.code)
	}
}

type ReplaceIO struct {
	orig *os.File
	in   *os.File
	out  *os.File
}

func newReplaceIO(orig *os.File) (*ReplaceIO, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	return &ReplaceIO{
		orig: orig,
		in:   r,
		out:  w,
	}, nil
}

func TestCli(t *testing.T) {
	stdin, err := newReplaceIO(os.Stdin)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = stdin.in
	_, err = stdin.out.WriteString(string(readFixture("test/fixture.json", t)))
	if err != nil {
		t.Fatal(err)
	}

	stdin.out.Close()

	stdout, err := newReplaceIO(os.Stdout)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = stdout.out
	var outputBuf bytes.Buffer
	cli([]string{"--code", postalCode().code, "--input=-", "--date", now().Format(time.DateOnly), "--hostname", "test"})
	os.Stdin = stdin.orig
	stdout.out.Close()
	_, err = io.Copy(&outputBuf, stdout.in)
	if err != nil {
		t.Fatal(err)
	}

	os.Stdout = stdout.orig
	expected := string(readFixture("test/fixture.ics", t))
	if outputBuf.String() != expected {
		t.Log(expected)
		t.Error(outputBuf.String())
	}
}

func TestDataURL(t *testing.T) { //nolint
	dataURL(postalCode())
}

func TestToPostalCode(t *testing.T) {
	for i := 1; i <= 9999; i++ {
		code := fmt.Sprintf("%04d", i)
		x, _ := toPostalCode(code)
		if x.code != code {
			t.Fatalf("Expected '%s', got '%s'", code, x.code)
		}
	}
}

func TestToPostalCodeOutOfRange(t *testing.T) {
	for _, i := range []int{0, 10000} {
		code := fmt.Sprintf("%d", i)
		_, err := toPostalCode(code)
		expected := fmt.Sprintf("invalid postal code: %04d", i)
		if err == nil || err.Error() != expected {
			t.Fatalf("Expected '%s', got '%s'", expected, err)
		}
	}
}
