package ical

import (
	"testing"
)

func TestIcalFieldStringWithAttributes(t *testing.T) {
	f := &Field{
		Name: "SUMMARY",
		Attributes: []Attribute{
			{Name: "X-A", Value: "12"},
		},
		Value: "Abba 12;\nHep stars 11",
	}
	if f.String() != "SUMMARY;X-A=12:Abba 12\\;\\nHep stars 11\r\n" {
		t.Errorf("%s", f.String())
	}
}

func TestIcalFieldStringWithoutAttributes(t *testing.T) {
	f := &Field{
		Name:       "SUMMARY",
		Attributes: []Attribute{},
		Value:      "Abba 12;\nHep stars 11",
	}
	if f.String() != "SUMMARY:Abba 12\\;\\nHep stars 11\r\n" {
		t.Errorf("%s", f.String())
	}
}
