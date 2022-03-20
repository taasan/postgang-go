package ical

import (
	"fmt"
	"net/url"
	"time"
)

type VEvent struct {
	uid     string
	url     *url.URL
	summary string
	date    *time.Time
}

func NewVEvent(uid string, u *url.URL, summary string, date *time.Time) *VEvent {
	return &VEvent{
		uid:     uid,
		url:     u,
		summary: summary,
		date:    date,
	}
}

type VCalendar struct {
	prodID    string
	events    []*VEvent
	timestamp *time.Time
}

func NewVCalendar(prodID string, timestamp *time.Time, events ...*VEvent) *VCalendar {
	return &VCalendar{prodID: prodID, events: events, timestamp: timestamp}
}

type icalField struct {
	name       string
	attributes []*Attribute
	value      string
}

type icalContent interface {
	fields() []*icalField
}

type Section struct {
	name    string
	content icalContent
}

func (section *Section) fields() []*icalField {
	buf := []*icalField{field("BEGIN", section.name)}
	buf = append(buf, section.content.fields()...)
	buf = append(buf, field("END", section.name))
	return buf
}

type Fields struct {
	Fields []*icalField
}

func (fields *Fields) fields() []*icalField {
	return fields.Fields
}

type Attribute struct {
	Name  string
	Value string
}

func dateAttribute() *Attribute {
	return &Attribute{
		Name:  "VALUE",
		Value: "DATE",
	}
}

func field(name, value string, attributes ...*Attribute) *icalField {
	return &icalField{
		name:       name,
		value:      fmt.Sprint(value),
		attributes: attributes,
	}
}

func urlField(name string, value *url.URL) *icalField {
	return field(name, value.String())
}

func dateField(name string, value *time.Time) *icalField {
	return field(name, value.Format("20060102"), dateAttribute())
}

func (event *VEvent) DtStart() *icalField {
	return dateField("DTSTART", event.date)
}

func (event *VEvent) DtEnd() *icalField {
	dtEnd := event.date.AddDate(0, 0, 1)
	return dateField("DTEND", &dtEnd)
}

func (cal *VCalendar) DtStamp() *icalField {
	return field("DTSTAMP", cal.timestamp.In(time.UTC).Format("20060102T150405Z"))
}

func (cal *VCalendar) ProdID() *icalField {
	return field("PRODID", cal.prodID)
}

func (event *VEvent) UID() *icalField {
	return field("UID", event.uid)
}

func (event *VEvent) URL() *icalField {
	return urlField("URL", event.url)
}

func (event *VEvent) Summary() *icalField {
	return field("SUMMARY", event.summary)
}

func Calendar(cal *VCalendar) *Section {
	fields := []*icalField{
		field("VERSION", "2.0"),
		cal.ProdID(),
		field("CALSCALE", "GREGORIAN"),
		field("METHOD", "PUBLISH"),
	}
	for _, x := range cal.events {
		e := event(x, cal)
		fields = append(fields, e.fields()...)
	}
	return section("VCALENDAR", &Fields{Fields: fields})
}

func event(event *VEvent, cal *VCalendar) *Section {
	fields := &Fields{
		Fields: []*icalField{
			event.UID(),
			event.URL(),
			event.Summary(),
			field("TRANSP", "TRANSPARENT"),
			event.DtStart(),
			event.DtEnd(),
			cal.DtStamp(),
		},
	}

	return section("VEVENT", fields)
}

func section(name string, content icalContent) *Section {
	return &Section{
		name:    name,
		content: content,
	}
}
