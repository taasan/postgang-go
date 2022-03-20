/*
Lag kalender fra postgangdata
*/
package main

import (
	"bufio"
	"bytes"
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

var baseURL = func() *url.URL {
	u, err := url.Parse("https://www.posten.no/levering-av-post/")
	if err != nil {
		log.Fatal(err)
	}
	return u
}()

var timezone = func() *time.Location {
	tz, err := time.LoadLocation("Europe/Oslo")
	if err != nil {
		log.Print("Unable to load time zone")
		panic(err)
	}
	return tz
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

func dataURL(code *postalCodeT) *url.URL {
	u, err := url.Parse(fmt.Sprintf("%s/_/component/main/1/leftRegion/1?postCode=%s", baseURL, code))
	if err != nil {
		log.Print("Unable to parse URL")
		panic(err)
	}
	return u
}

func readData(now *time.Time, in io.Reader) (*postenResponseT, *time.Time, error) {
	bodyString, err := io.ReadAll(in)
	if err != nil {
		return nil, nil, err
	}
	var data postenResponseT
	err = json.NewDecoder(bytes.NewReader(bodyString)).Decode(&data)
	if err != nil {
		return nil, nil, err
	}
	return &data, now, nil
}

func fetchData(postalCode *postalCodeT, timezone *time.Location) (*postenResponseT, *time.Time, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", dataURL(postalCode).String(), http.NoBody)
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
		return nil, nil, fmt.Errorf("unable to parse JSON: %s", err)
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
		log.Fatalf("No match: %s", s)
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

type calendarT struct {
	now      *time.Time
	dates    []*time.Time
	prodID   string
	hostname string
	code     *postalCodeT
}

func toCalendarT(now *time.Time, response *postenResponseT, hostname string, postalCode *postalCodeT) *calendarT {
	buf := make([]*time.Time, len(response.NextDeliveryDays))
	for i, x := range response.NextDeliveryDays {
		buf[i] = parseDeliveryDay(x, now.Location(), postalCode).toDate(now)
	}
	return &calendarT{
		dates:    buf,
		now:      now,
		prodID:   fmt.Sprintf("-//Aasan//Aasan Go Postgang %s@%s//EN", postalCode, version),
		hostname: hostname,
		code:     postalCode,
	}
}

func toVCalendar(cal *calendarT) *ical.Section {
	buf := make([]*ical.VEvent, len(cal.dates))
	for i, x := range cal.dates {
		buf[i] = toVEvent(x, cal)
	}
	return ical.Calendar(ical.NewVCalendar(cal.prodID, cal.now, buf...))
}

func toVEvent(date *time.Time, cal *calendarT) *ical.VEvent {
	dayName := weekdayNames[date.Weekday()]
	dayNum := date.Day()
	return ical.NewVEvent(
		fmt.Sprintf("postgang-%s@%s", date.Format("20060102"), cal.hostname),
		baseURL,
		fmt.Sprintf("%s: Posten kommer %s %d.", cal.code, dayName, dayNum),
		date,
	)
}

type postalCodeT struct {
	code string
}

func (c *postalCodeT) String() string {
	return c.code
}

func toPostalCode(s string) (*postalCodeT, error) {
	x, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return nil, err
	}
	var postalCode postalCodeT
	if x < 1 || x > maxPostalCode {
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
	commit, err := base64.StdEncoding.DecodeString(gitCommit)
	if err == nil {
		fmt.Fprint(wr, string(commit))
	} else {
		log.Print("Error base64 decoding git commit", err, gitCommit)
	}
}

func die(msg interface{}) {
	printVersion(os.Stderr)
	fmt.Fprintln(os.Stderr)
	log.Panic(msg)
}

type commandLineArgs struct {
	code       *postalCodeT
	outputPath string
	fetch      func() (*postenResponseT, *time.Time, error)
	err        error
	version    bool
	hostname   string
}

func parseArgs(cmd *flag.FlagSet, a []string) (commandLineArgs, error) {
	var (
		codeArg       string
		outputPathArg string
		versionArg    bool
		inputPathArg  string
		dateArg       string
		hostnameArg   string
	)
	cmd.StringVar(&inputPathArg, "input", "", "Read input from `file` instead of fetching from posten.no")
	cmd.StringVar(&dateArg, "date", "", "Use as fetch `date`")
	cmd.StringVar(&hostnameArg, "hostname", "", "Use in UID")
	cmd.BoolVar(&versionArg, "version", false, "Show version and exit")
	cmd.StringVar(&codeArg, "code", "", "Postal code, an `integer` between 1 and 9999")
	cmd.StringVar(&outputPathArg, "output", "", "Path of output file")
	if err := cmd.Parse(a); err != nil {
		return commandLineArgs{}, err
	}
	if versionArg {
		return commandLineArgs{version: true}, nil
	}
	postalCode, err := toPostalCode(codeArg)
	if err != nil {
		return commandLineArgs{}, err
	}

	var doFetch func() (*postenResponseT, *time.Time, error)
	if inputPathArg != "" {
		var in *os.File
		if inputPathArg == "-" {
			in = os.Stdin
		} else {
			in, err = os.Open(inputPathArg)
			if err != nil {
				return commandLineArgs{}, err
			}
		}
		var now time.Time
		if dateArg != "" {
			now, err = time.Parse("2006-01-02", dateArg)
			if err != nil {
				return commandLineArgs{}, err
			}
		} else {
			now = time.Now()
		}
		now = now.In(timezone)
		doFetch = func() (*postenResponseT, *time.Time, error) {
			return readData(&now, in)
		}
	} else {
		doFetch = func() (*postenResponseT, *time.Time, error) {
			return fetchData(postalCode, timezone)
		}
	}
	if outputPathArg == "-" {
		outputPathArg = ""
	}
	return commandLineArgs{
		code:       postalCode,
		fetch:      doFetch,
		outputPath: outputPathArg,
		version:    versionArg,
		err:        err,
		hostname:   hostnameArg,
	}, nil
}

func cli(as []string) {
	args, err := parseArgs(flag.CommandLine, as)
	if err != nil {
		die(err)
	}
	if args.version {
		printVersion(os.Stdout)
		os.Exit(0)
	}
	wr := os.Stdout
	ok := false
	if args.outputPath != "" {
		var tmpFile, outputDestination *os.File
		outputDestination, err = os.Create(args.outputPath)
		if err != nil {
			die(err)
		}
		tmpFile, err = ioutil.TempFile("", "postgang-")
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
	var response *postenResponseT
	var now *time.Time
	response, now, err = args.fetch()
	if err != nil {
		die(err)
	}
	if response.IsStreetAddressReq {
		die(fmt.Sprintf("Street address is required %+v", response))
	}
	var hostname string
	if args.hostname != "" {
		hostname = args.hostname
	} else {
		hostname, err = os.Hostname()
		if err != nil {
			hostname = err.Error()
		}
	}
	calendar := toCalendarT(now, response, hostname, args.code)
	if len(calendar.dates) == 0 {
		die(fmt.Sprintf("No delivery days found, check postal code: %s", args.code))
	}
	buf := bufio.NewWriter(wr)
	defer buf.Flush()

	p := ical.NewContentPrinter(buf, true).Print(toVCalendar(calendar))
	err = p.Error()
	if err != nil {
		die(err)
	}
	ok = true // Used in closure
}

func main() {
	cli(os.Args[1:])
}
