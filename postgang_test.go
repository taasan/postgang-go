package main

import (
	"fmt"
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

func TestFromMonthName(t *testing.T) {
	for k, v := range months {
		if k != monthNames[v] {
			t.Fatalf("%s => %s", k, v)
		}
	}
}

func TestFromMonth(t *testing.T) {
	for k, v := range monthNames {
		if k != months[v] {
			t.Fatalf("%s => %s", k, v)
		}
	}
}

func now() *time.Time {
	date, _ := time.Parse(time.RFC1123, "Tue, 28 Dec 2021 23:36:55 GMT")
	return &date
}

func prodID() string {
	return fmt.Sprintf("-//Aasan//Aasan Go Postgang %s//EN", version)
}

func postalCode() *postalCodeT {
	postalCode, _ := toPostalCode(6666)
	return postalCode
}

func event(date time.Time) eventT {
	return eventT{
		Date:       date,
		PostalCode: *postalCode(),
	}
}

func fetchDataFixture() (*postenResponseT, *time.Time) {
	return &postenResponseT{
		NextDeliveryDays: []string{
			"i dag tirsdag 28. desember",
			"i morgen onsdag 29. desember",
			"torsdag 30. desember",
			"fredag 31. desember",
			"lørdag 1. januar",
			"søndag 2. januar",
			"mandag 3. januar",
		},
		IsStreetAddressReq: false,
	}, now()
}

func calendarTFixture() calendarT {
	timezone := now().Location()
	events := []eventT{
		event(time.Date(2021, 12, 28, 0, 0, 0, 0, timezone)),
		event(time.Date(2021, 12, 29, 0, 0, 0, 0, timezone)),
		event(time.Date(2021, 12, 30, 0, 0, 0, 0, timezone)),
		event(time.Date(2021, 12, 31, 0, 0, 0, 0, timezone)),
		event(time.Date(2022, 1, 1, 0, 0, 0, 0, timezone)),
		event(time.Date(2022, 1, 2, 0, 0, 0, 0, timezone)),
		event(time.Date(2022, 1, 3, 0, 0, 0, 0, timezone)),
	}
	return calendarT{
		Now:      *now(),
		ProdID:   prodID(),
		Events:   events,
		Hostname: "test",
	}

}

func TestToCalendarT(t *testing.T) {
	resp, now := fetchDataFixture()
	hostname := "test"
	calendar := *toCalendarT(now, resp, hostname, postalCode())
	expectedCalendar := calendarTFixture()
	if !reflect.DeepEqual(calendar, expectedCalendar) {
		t.Fatalf("%s != %s", calendar, expectedCalendar)

	}
}

/*
{2021-12-28 23:36:55 +0000 GMT [{2021-12-28 00:00:00 +0000 GMT 6666} {2021-12-29 00:00:00 +0000 GMT 6666} {2021-12-30 00:00:00 +0000 GMT 6666} {2021-12-31 00:00:00 +0000 GMT 6666} {2022-01-01 00:00:00 +0000 GMT 6666} {2022-01-02 00:00:00 +0000 GMT 6666} {2022-01-03 00:00:00 +0000 GMT 6666}] -//Aasan//Aasan Go Postgang v1.0.0//EN test}
{2021-12-28 23:36:55 +0000 GMT [{2021-12-28 00:00:00 +0100 CET 6666} {2021-12-29 00:00:00 +0100 CET 6666} {2021-12-30 00:00:00 +0100 CET 6666} {2021-12-31 00:00:00 +0100 CET 6666} {2022-01-01 00:00:00 +0100 CET 6666} {2022-01-02 00:00:00 +0100 CET 6666} {2022-01-03 00:00:00 +0100 CET 6666}] -//Aasan//Aasan Go Postgang v1.0.0//EN test}
*/
