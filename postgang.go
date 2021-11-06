/*
Lag kalender fra postgangdata
*/
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
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
	PostalCode postalCodeT
	Day        time.Weekday
	DayNum     int
	Month      time.Month
	Timezone   time.Location
}

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

const urlTemplate = "https://www.posten.no/levering-av-post/_/component/main/1/leftRegion/1?postCode=%s"

func fetchData(postalCode *postalCodeT, timezone *time.Location) (*postenResponseT, *time.Time, error) {
	url := fmt.Sprintf(urlTemplate, postalCode)
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
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

func parseDeliveryDay(s string, tz time.Location, postalCode *postalCodeT) *deliveryDayT {
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
		PostalCode: *postalCode,
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
	Date       time.Time
	PostalCode postalCodeT
}

type calendarT struct {
	Now      time.Time
	Events   []eventT
	ProdID   string
	Hostname string
}

func (day *deliveryDayT) toEventT(now *time.Time) eventT {
	date := (*day.toDate(now))

	data := eventT{
		Date:       date,
		PostalCode: day.PostalCode,
	}
	return data
}

func toCalendarT(now *time.Time, response *postenResponseT, hostname string, postalCode *postalCodeT) *calendarT {
	buf := make([]eventT, len(response.NextDeliveryDays))
	for i, x := range response.NextDeliveryDays {
		buf[i] = parseDeliveryDay(x, *now.Location(), postalCode).toEventT(now)
	}
	return &calendarT{
		Events:   buf,
		Now:      *now,
		ProdID:   "-//Aasan//Aasan Go Postgang v1.0.0//EN",
		Hostname: hostname,
	}
}

func toVCalendar(cal *calendarT) []ical.Field {
	buf := []ical.Field{
		ical.New("VERSION", "2.0"),
		ical.New("PRODID", cal.ProdID),
		ical.New("CALSCALE", "GREGORIAN"),
		ical.New("METHOD", "PUBLISH"),
	}
	for _, x := range cal.Events {
		buf = append(buf, toVEvent(x, cal.Hostname, cal.Now)...)
	}
	return ical.Section("VCALENDAR", buf)
}

func toVEvent(event eventT, hostname string, now time.Time) []ical.Field {
	fields := []ical.Field{
		{
			Name:  "UID",
			Value: fmt.Sprintf("DeliveryDay %s (%s) @%s", event.Date.Format("20060102"), event.PostalCode, hostname),
		},
		{
			Name:  "ORGANIZER",
			Value: fmt.Sprintf("Posten %s", event.PostalCode),
		},
		{
			Name:  "SUMMARY",
			Value: fmt.Sprintf("Posten kommer %s %d.", weekdayNames[event.Date.Weekday()], event.Date.Day()),
		},
		ical.DtStart(event.Date),
		ical.DtEnd(event.Date.AddDate(0, 0, 1)),
		ical.DtStamp(now),
	}
	return ical.Section("VEVENT", fields)
}

type postalCodeT struct {
	code string
}

func (c postalCodeT) String() string {
	return c.code
}

func toPostalCode(x uint) (*postalCodeT, error) {
	var postalCode postalCodeT
	if x > 9999 {
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

func printVersionLine(key string, value string) {
	fmt.Printf("%-12s: %s", key, value)
	fmt.Println()
}

func main() {
	var (
		codeArg       uint
		outputPathArg string
		versionArg    bool
	)
	flag.BoolVar(&versionArg, "version", false, "Show version and exit")
	flag.UintVar(&codeArg, "code", 7530, "Postal code")
	flag.StringVar(&outputPathArg, "output", "", "Path of output file")
	flag.Parse()
	if versionArg {
		printVersionLine("Build date", buildstamp)
		printVersionLine("Version", version)
		fmt.Println()
		commit, err := base64.RawStdEncoding.DecodeString(gitCommit)
		if err == nil {
			fmt.Print(string(commit))
		}
		os.Exit(0)
	}
	postalCode, err := toPostalCode(codeArg)
	if err != nil {
		log.Fatal(err)
	}
	wr := os.Stdout
	ok := false
	if outputPathArg != "" {
		tmpFile, err := ioutil.TempFile("", "postgang-")
		if err != nil {
			log.Fatal(err)
		}
		outputDestination, err := os.Create(outputPathArg)
		if err != nil {
			log.Fatal(err)
		}
		wr = tmpFile
		defer func() {
			if ok {
				err = copyFile(tmpFile.Name(), outputDestination)
				if err != nil {
					log.Fatalf("CopyFile failed: %s", err)
				}
			}
			os.Remove(tmpFile.Name())
		}()
	}
	tz, err := time.LoadLocation("Europe/Oslo")
	if err != nil {
		log.Print(err)
	} else {
		tz = time.Local
	}
	response, now, err := fetchData(postalCode, tz)
	if err != nil {
		log.Fatal(err)
	}
	if response.IsStreetAddressReq {
		log.Fatalf("Street address is required %+v", response)
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = err.Error()
	}
	calendar := toCalendarT(now, response, hostname, postalCode)
	if len(calendar.Events) == 0 {
		log.Fatalf("No delivery days found, check postal code: %s", postalCode)
	}
	_, err = ical.WriteIcal(wr, toVCalendar(calendar)...)
	if err != nil {
		log.Fatal(err)
	}
	ok = true
}
