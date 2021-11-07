package ical

import (
	"fmt"
	"net/url"
	"time"
)

const maxLineLen = 75

type VEvent struct {
	UID     string
	URL     *url.URL
	Summary string
	Date    *time.Time
}

type VCalendar struct {
	ProdID    string
	Events    []*VEvent
	Timestamp *time.Time
}

func NewVCalendar(prodID string, timestamp *time.Time, events ...*VEvent) *VCalendar {
	return &VCalendar{ProdID: prodID, Events: events, Timestamp: timestamp}
}

type icalField struct {
	name       string
	attributes []*Attribute
	value      string
}

type icalContent interface {
	getFields() []*icalField
}

type Section struct {
	name       string
	attributes []*Attribute
	content    icalContent
}

func (section *Section) getFields() []*icalField {
	buf := []*icalField{field("BEGIN", section.name, section.attributes...)}
	buf = append(buf, section.content.getFields()...)
	buf = append(buf, field("END", section.name))
	return buf
}

type Fields struct {
	Fields []*icalField
}

func (fields *Fields) getFields() []*icalField {
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

func DtStart(value *time.Time) *icalField {
	return dateField("DTSTART", value)
}

func DtEnd(value *time.Time) *icalField {
	return dateField("DTEND", value)
}

func DtStamp(value *time.Time) *icalField {
	return field("DTSTAMP", value.Format("20060102T150405Z"))
}

func Version() *icalField {
	return field("VERSION", "2.0")
}

func ProdID(value string) *icalField {
	return field("PRODID", value)
}

func CalScale() *icalField {
	return field("CALSCALE", "GREGORIAN")
}

func Method() *icalField {
	return field("METHOD", "PUBLISH")
}

func Transp() *icalField {
	return field("TRANSP", "TRANSPARENT")
}

func UID(value string) *icalField {
	return field("UID", value)
}

func URL(value *url.URL) *icalField {
	return urlField("URL", value)
}

func Summary(value string) *icalField {
	return field("SUMMARY", value)
}

func Calendar(cal *VCalendar) *Section {
	fields := []*icalField{
		Version(),
		ProdID(cal.ProdID),
		CalScale(),
		Method(),
	}
	for _, x := range cal.Events {
		e := event(x, cal.Timestamp)
		fields = append(fields, e.getFields()...)
	}
	return section("VCALENDAR", &Fields{Fields: fields})
}

func event(event *VEvent, now *time.Time) *Section {
	dtEnd := event.Date.AddDate(0, 0, 1)
	fields := &Fields{
		Fields: []*icalField{
			UID(event.UID),
			URL(event.URL),
			Summary(event.Summary),
			Transp(),
			DtStart(event.Date),
			DtEnd(&dtEnd),
			DtStamp(now),
		},
	}

	return section("VEVENT", fields)
}

func section(name string, content icalContent, attributes ...*Attribute) *Section {
	return &Section{
		name:       name,
		content:    content,
		attributes: attributes,
	}
}
