package ical

import (
	"net/url"
	"testing"
	"time"
)

func timestamp() *time.Time {
	t := time.Date(2020, 1, 2, 3, 4, 5, 6, time.Local)
	return &t
}

func prodID() string {
	return "prodID"
}

func veventFixture() *VEvent {
	u, _ := url.Parse("https://www.example.com")
	return &VEvent{
		UID:     "UID",
		URL:     u,
		Summary: "Summary",
		Date:    timestamp(),
	}
}

func vcalFixture() *VCalendar {
	return NewVCalendar(prodID(), timestamp(), veventFixture())
}

func TestCalendar(t *testing.T) {
	cal := Calendar(vcalFixture())
	expected := &Section{
		name:    "VCALENDAR",
		content: &Fields{Fields: make([]*icalField, 13)},
	}
	if cal.name != expected.name {
		t.Logf("%s != %s", cal.name, expected.name)
		t.Fail()
	}
	gotLen := len(cal.content.getFields())
	expectedLen := len(expected.content.getFields())
	if gotLen != expectedLen {
		t.Logf("Expected %d fields, got %d", expectedLen, gotLen)
		t.Fail()
	}
}
