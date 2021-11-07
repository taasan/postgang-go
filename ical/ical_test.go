package ical

import (
	"testing"
)

func TestIcalFieldStringWithAttributes(t *testing.T) {
	f := &icalField{
		name: "SUMMARY",
		attributes: []*Attribute{
			{Name: "X-A", Value: "12"},
		},
		value: "Abba 12;\nHep stars 11",
	}
	if f.String() != "SUMMARY;X-A=12:Abba 12\\;\\nHep stars 11\r\n" {
		t.Errorf("%s", f.String())
	}
}

func TestIcalFieldStringWithoutAttributes(t *testing.T) {
	f := &icalField{
		name:       "SUMMARY",
		attributes: []*Attribute{},
		value:      "Abba 12;\nHep stars 11",
	}
	if f.String() != "SUMMARY:Abba 12\\;\\nHep stars 11\r\n" {
		t.Errorf("%s", f.String())
	}
}
