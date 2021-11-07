/*
Lag kalender fra postgangdata
*/
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/taasan/postgang/ical"
)

type postenResponseT struct {
	NextDeliveryDays   []string `json:"nextDeliveryDays"`
	IsStreetAddressReq bool     `json:"isStreetAddressReq"`
}

type deliveryDayT struct {
	PostalCode *postalCodeT
	Day        time.Weekday
	DayNum     int
	Month      time.Month
	Timezone   *time.Location
}

const maxPostalCode = 9999
const meraker = 7530

var baseURL = func() *url.URL {
	u, err := url.Parse("https://www.posten.no/levering-av-post/")
	if err != nil {
		log.Fatal(err)
	}
	return u
}()

var version = "development"
var buildstamp = ""
var gitCommit = ""

var weekdays = map[string]time.Weekday{
	"mandag":  time.Monday,
	"tirsdag": time.Tuesday,
	"onsdag":  time.Wednesday,
	"torsdag": time.Thursday,
	"fredag":  time.Friday,
	"lørdag":  time.Saturday,
	"søndag":  time.Sunday,
}

var weekdayNames = map[time.Weekday]string{
	time.Monday:    "mandag",
	time.Tuesday:   "tirsdag",
	time.Wednesday: "onsdag",
	time.Thursday:  "torsdag",
	time.Friday:    "fredag",
	time.Saturday:  "lørdag",
	time.Sunday:    "søndag",
}

var months = map[string]time.Month{
	"januar":    time.January,
	"februar":   time.February,
	"mars":      time.March,
	"april":     time.April,
	"mai":       time.May,
	"juni":      time.June,
	"juli":      time.July,
	"august":    time.August,
	"september": time.September,
	"oktober":   time.October,
	"november":  time.November,
	"desember":  time.December,
}

var monthNames = map[time.Month]string{
	time.January:   "januar",
	time.February:  "februar",
	time.March:     "mars",
	time.April:     "april",
	time.May:       "mai",
	time.June:      "juni",
	time.July:      "juli",
	time.August:    "august",
	time.September: "september",
	time.October:   "oktober",
	time.November:  "november",
	time.December:  "desember",
}

var deliverydayRe = func() *regexp.Regexp {
	buf := make([]string, len(months))
	for v := range months {
		buf = append(buf, v)
	}
	months := strings.Join(buf, "|")
	buf = make([]string, len(weekdayNames))
	for v := range weekdays {
		buf = append(buf, v)
	}
	days := strings.Join(buf, "|")
	return regexp.MustCompile(fmt.Sprintf(`^(?:i (?:dag|morgen) )?(?P<dayname>%s) (?P<day>\d+)\. (?P<month>%s)$`, days, months))
}()

func dataURL(code postalCodeT) string {
	return fmt.Sprintf("%s/_/component/main/1/leftRegion/1?postCode=%s", baseURL, code)
}

func fetchData(postalCode *postalCodeT, timezone *time.Location) (*postenResponseT, *time.Time, error) {
	u := dataURL(*postalCode)
	client := &http.Client{}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Add("x-requested-with", "XMLHttpRequest")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("got HTTP error: %s", resp.Status)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	bodyString := string(bodyBytes)

	var data postenResponseT
	err = json.NewDecoder(strings.NewReader(bodyString)).Decode(&data)
	if err != nil {
		return nil, nil, err
	}

	now, err := time.Parse(time.RFC1123, resp.Header.Get("date"))
	if err != nil {
		log.Println(err)
		now = time.Now()
	}
	now = now.In(timezone)
	return &data, &now, nil
}

func parseDeliveryDay(s string, tz *time.Location, postalCode *postalCodeT) *deliveryDayT {
	match := deliverydayRe.FindStringSubmatch(s)
	if match == nil {
		log.Fatal("No match")
	}
	dayNum, _ := strconv.Atoi(match[2])
	return &deliveryDayT{
		Day:        weekdays[match[1]],
		DayNum:     dayNum,
		Month:      months[match[3]],
		Timezone:   tz,
		PostalCode: postalCode,
	}
}

func (day *deliveryDayT) toDate(now *time.Time) *time.Time {
	year := now.Year()
	month := now.Month()
	if month == time.December && day.Month != month {
		year++
	}

	date := time.Date(year, day.Month, day.DayNum, 0, 0, 0, 0, now.Location())
	if date.Weekday() != day.Day {
		// Sanity check
		log.Fatalf("Weekday mismatch: %+v %+v", day, date)
	}
	return &date
}

type eventT struct {
	Date       *time.Time
	PostalCode *postalCodeT
}

type calendarT struct {
	Now      *time.Time
	Events   []eventT
	ProdID   string
	Hostname string
}

func (day *deliveryDayT) toEventT(now *time.Time) eventT {
	date := day.toDate(now)

	data := eventT{
		Date:       date,
		PostalCode: day.PostalCode,
	}
	return data
}

func toCalendarT(now *time.Time, response *postenResponseT, hostname string, postalCode *postalCodeT) *calendarT {
	buf := make([]eventT, len(response.NextDeliveryDays))
	for i, x := range response.NextDeliveryDays {
		buf[i] = parseDeliveryDay(x, now.Location(), postalCode).toEventT(now)
	}
	return &calendarT{
		Events:   buf,
		Now:      now,
		ProdID:   fmt.Sprintf("-//Aasan//Aasan Go Postgang %s@%s//EN", postalCode, version),
		Hostname: hostname,
	}
}

func toVCalendar(cal *calendarT) *ical.Section {
	buf := make([]*ical.VEvent, len(cal.Events))
	for i, x := range cal.Events {
		buf[i] = toVEvent(x, cal.Hostname)
	}
	return ical.Calendar(ical.NewVCalendar(cal.ProdID, buf))
}

func toVEvent(event eventT, hostname string) *ical.VEvent {
	dayName := weekdayNames[event.Date.Weekday()]
	dayNum := event.Date.Day()
	monthName := monthNames[event.Date.Month()]
	return &ical.VEvent{
		UID: fmt.Sprintf(
			"DeliveryDay {day = %s, dayNum = %d, month = %s}@%s",
			dayName,
			dayNum,
			monthName,
			hostname,
		),
		URL:     baseURL,
		Summary: fmt.Sprintf("Posten kommer %s %d.", dayName, dayNum),
		Date:    event.Date,
	}
}

type postalCodeT struct {
	code string
}

func (c postalCodeT) String() string {
	return c.code
}

func toPostalCode(x uint) (*postalCodeT, error) {
	var postalCode postalCodeT
	if x > maxPostalCode {
		return &postalCode, fmt.Errorf("invalid postal code: %04d", x)
	}
	return &postalCodeT{fmt.Sprintf("%04d", x)}, nil
}

func copyFile(sourcePath string, dest io.Writer) error {
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("couldn't open source file: %s", err)
	}
	defer inputFile.Close()
	_, err = io.Copy(dest, inputFile)
	if err != nil {
		return fmt.Errorf("writing failed: %s", err)
	}
	return nil
}

func printVersionLine(wr io.Writer, key, value string) {
	fmt.Fprintf(wr, "%-12s: %s", key, value)
	fmt.Fprintln(wr)
}

func printVersion(wr io.Writer) {
	printVersionLine(wr, "Build date", buildstamp)
	printVersionLine(wr, "Version", version)
	fmt.Fprintln(wr)
	commit, err := base64.RawStdEncoding.DecodeString(gitCommit)
	if err == nil {
		fmt.Fprint(wr, string(commit))
	}
}

func die(msg interface{}) {
	printVersion(os.Stderr)
	fmt.Fprintln(os.Stderr)
	log.Fatal(msg)
}

func main() {
	var (
		codeArg       uint
		outputPathArg string
		versionArg    bool
	)
	flag.BoolVar(&versionArg, "version", false, "Show version and exit")
	flag.UintVar(&codeArg, "code", meraker, "Postal code")
	flag.StringVar(&outputPathArg, "output", "", "Path of output file")
	flag.Parse()
	if versionArg {
		printVersion(os.Stdout)
		os.Exit(0)
	}
	postalCode, err := toPostalCode(codeArg)
	if err != nil {
		die(err)
	}
	wr := os.Stdout
	ok := false
	if outputPathArg != "" {
		var tmpFile, outputDestination *os.File
		tmpFile, err = ioutil.TempFile("", "postgang-")
		if err != nil {
			die(err)
		}
		outputDestination, err = os.Create(outputPathArg)
		if err != nil {
			die(err)
		}
		wr = tmpFile
		defer func() {
			if ok {
				err = copyFile(tmpFile.Name(), outputDestination)
				if err != nil {
					die(err)
				}
			}
			os.Remove(tmpFile.Name())
		}()
	}
	var tz *time.Location
	tz, err = time.LoadLocation("Europe/Oslo")
	if err != nil {
		log.Print(err)
	} else {
		tz = time.Local
	}
	var response *postenResponseT
	var now *time.Time
	response, now, err = fetchData(postalCode, tz)
	if err != nil {
		die(err)
	}
	if response.IsStreetAddressReq {
		die(fmt.Sprintf("Street address is required %+v", response))
	}
	var hostname string
	hostname, err = os.Hostname()
	if err != nil {
		hostname = err.Error()
	}
	calendar := toCalendarT(now, response, hostname, postalCode)
	if len(calendar.Events) == 0 {
		die(fmt.Sprintf("No delivery days found, check postal code: %s", postalCode))
	}
	buf := bufio.NewWriter(wr)
	defer buf.Flush()

	_, err = toVCalendar(calendar).Print(ical.NewContentPrinter(buf, true))
	if err != nil {
		die(err)
	}
	ok = true // Used in closure
}
