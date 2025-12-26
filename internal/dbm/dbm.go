//nolint:tagliatelle
package dbm

import "encoding/xml"

type DBModel struct {
	XMLName   xml.Name   `xml:"dbmodel"`
	Roles     []Role     `xml:"role"`
	Databases []Database `xml:"database"`
}

type Role struct {
	Name        string `xml:"name,attr"`
	SQLDisabled bool   `xml:"sql-disabled,attr"`
}

type Database struct {
	Name        string `xml:"name,attr"`
	SQLDisabled bool   `xml:"sql-disabled,attr"`
}
