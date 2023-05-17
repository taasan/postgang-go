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
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/taasan/postgang/ical"
)

type CivilTime struct {
	time *time.Time
}

func (c *CivilTime) UnmarshalJSON(b []byte) error {
	value := strings.Trim(string(b), `"`)
	if value == "" || value == "null" {
		return nil
	}

	t, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return err
	}
	*c = CivilTime{time: &t}
	return nil
}

type postenResponseT struct {
	DeliveryDates []*CivilTime `json:"delivery_dates"`
}

const maxPostalCode = 9999

var baseURL = func() *url.URL {
	if u, err := url.Parse("https://www.posten.no/levering-av-post/"); err != nil {
		panic(err)
	} else {
		return u
	}
}()

var timezone = func() *time.Location {
	if tz, err := time.LoadLocation("Europe/Oslo"); err != nil {
		log.Print("Unable to load time zone")
		panic(err)
	} else {
		return tz
	}
}()

var version = "development"
var buildstamp = ""
var gitCommit = ""

func reverseMap[K comparable, V comparable](m map[K]V) map[V]K {
	n := make(map[V]K, len(m))
	for k, v := range m {
		n[v] = k
	}
	return n
}

var weekdays = map[string]time.Weekday{
	"mandag":  time.Monday,
	"tirsdag": time.Tuesday,
	"onsdag":  time.Wednesday,
	"torsdag": time.Thursday,
	"fredag":  time.Friday,
	"lørdag":  time.Saturday,
	"søndag":  time.Sunday,
}

var weekdayNames = reverseMap(weekdays)

func dataURL(code *postalCodeT) *url.URL {
	if u, err := url.Parse(fmt.Sprintf("https://api.bring.com/address/api/no/postal-codes/%s/mailbox-delivery-dates", code)); err != nil {
		log.Print("Unable to parse URL")
		panic(err)
	} else {
		return u
	}
}

func readData(now *time.Time, in io.Reader) (*postenResponseT, *time.Time, error) {
	if bodyString, err := io.ReadAll(in); err != nil {
		return nil, nil, err
	} else {
		var data postenResponseT
		if err := json.NewDecoder(bytes.NewReader(bodyString)).Decode(&data); err != nil {
			return nil, nil, err
		}
		return &data, now, nil
	}
}

type credentials struct {
	uid string
	key string
}

func fetchData(postalCode *postalCodeT, timezone *time.Location, creds *credentials) (*postenResponseT, *time.Time, error) {
	client := &http.Client{}
	if req, err := http.NewRequest("GET", dataURL(postalCode).String(), http.NoBody); err != nil {
		return nil, nil, err
	} else {
		req.Header.Add("X-Mybring-API-Key", creds.key)
		req.Header.Add("X-Mybring-API-Uid", creds.uid)

		if resp, err := client.Do(req); err != nil {
			return nil, nil, err
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return nil, nil, fmt.Errorf("got HTTP error: %s", resp.Status)
			}
			if bodyBytes, err := io.ReadAll(resp.Body); err != nil {
				return nil, nil, err
			} else {
				bodyString := string(bodyBytes)
				var data postenResponseT
				if err = json.NewDecoder(strings.NewReader(bodyString)).Decode(&data); err != nil {
					return nil, nil, fmt.Errorf("unable to parse JSON: %w", err)
				}
				var now time.Time
				if now, err = time.Parse(time.RFC1123, resp.Header.Get("date")); err != nil {
					log.Println(err)
					now = time.Now()
				}
				now = now.In(timezone)
				return &data, &now, nil
			}
		}
	}
}

type calendarT struct {
	now      *time.Time
	dates    []*CivilTime
	prodID   string
	hostname string
	code     *postalCodeT
}

func toCalendarT(now *time.Time, response *postenResponseT, hostname string, postalCode *postalCodeT) *calendarT {
	return &calendarT{
		dates:    response.DeliveryDates,
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

func toVEvent(date *CivilTime, cal *calendarT) *ical.VEvent {
	dayName := weekdayNames[date.time.Weekday()]
	dayNum := date.time.Day()
	return ical.NewVEvent(
		fmt.Sprintf("postgang-%s@%s", date.time.Format("20060102"), cal.hostname),
		baseURL,
		fmt.Sprintf("%s: Posten kommer %s %d.", cal.code, dayName, dayNum),
		date.time,
	)
}

type postalCodeT struct {
	code string
}

func (c *postalCodeT) String() string {
	return c.code
}

func toPostalCode(s string) (*postalCodeT, error) {
	if x, err := strconv.ParseUint(s, 10, 16); err != nil {
		return nil, err
	} else {
		var postalCode postalCodeT
		if x < 1 || x > maxPostalCode {
			return &postalCode, fmt.Errorf("invalid postal code: %04d", x)
		}
		return &postalCodeT{fmt.Sprintf("%04d", x)}, nil
	}
}

func copyFile(sourcePath string, dest io.Writer) error {
	if inputFile, err := os.Open(sourcePath); err != nil {
		return fmt.Errorf("couldn't open source file: %w", err)
	} else {
		defer inputFile.Close()
		if _, err = io.Copy(dest, inputFile); err != nil {
			return fmt.Errorf("writing failed: %w", err)
		}
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
	if commit, err := base64.StdEncoding.DecodeString(gitCommit); err == nil {
		fmt.Fprint(wr, string(commit))
	} else {
		log.Print("Error base64 decoding git commit", err, gitCommit)
	}
}

func die(msg any) {
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
	if postalCode, err := toPostalCode(codeArg); err != nil {
		return commandLineArgs{}, err
	} else {
		var doFetch func() (*postenResponseT, *time.Time, error)
		if inputPathArg != "" {
			var in *os.File
			if inputPathArg == "-" {
				in = os.Stdin
			} else {
				if in, err = os.Open(inputPathArg); err != nil {
					return commandLineArgs{}, err
				}
			}
			var now time.Time
			if dateArg != "" {
				if now, err = time.Parse(time.DateOnly, dateArg); err != nil {
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
				uid := os.Getenv("POSTGANG_API_UID")
				if uid == "" {
					return nil, nil, fmt.Errorf("POSTGANG_API_UID not set")
				}
				key := os.Getenv("POSTGANG_API_KEY")
				if key == "" {
					return nil, nil, fmt.Errorf("POSTGANG_API_KEY not set")
				}
				creds := &credentials{uid, key}
				return fetchData(postalCode, timezone, creds)
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
}

func cli(as []string) {
	if args, err := parseArgs(flag.CommandLine, as); err != nil {
		die(err)
	} else {
		if args.version {
			printVersion(os.Stdout)
			os.Exit(0)
		}
		wr := os.Stdout
		ok := false
		if args.outputPath != "" {
			var tmpFile, outputDestination *os.File
			if outputDestination, err = os.Create(args.outputPath); err != nil {
				die(err)
			}
			if tmpFile, err = os.CreateTemp("", "postgang-"); err != nil {
				die(err)
			}
			wr = tmpFile
			defer func() {
				if ok {
					if err = copyFile(tmpFile.Name(), outputDestination); err != nil {
						die(err)
					}
				}
				os.Remove(tmpFile.Name())
			}()
		}
		var response *postenResponseT
		var now *time.Time
		if response, now, err = args.fetch(); err != nil {
			die(err)
		}
		var hostname string
		if args.hostname != "" {
			hostname = args.hostname
		} else {
			if hostname, err = os.Hostname(); err != nil {
				hostname = err.Error()
			}
		}
		calendar := toCalendarT(now, response, hostname, args.code)
		if len(calendar.dates) == 0 {
			die(fmt.Sprintf("No delivery days found, check postal code: %s", args.code))
		}
		buf := bufio.NewWriter(wr)
		defer buf.Flush()

		p := ical.NewContentPrinter(buf).Print(toVCalendar(calendar))
		if err = p.Error(); err != nil {
			die(err)
		}
		ok = true // Used in closure
	}
}

func main() {
	cli(os.Args[1:])
}
