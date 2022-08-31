//nolint:tagliatelle
package dbm

import "encoding/xml"

type DBModel struct {
	XMLName xml.Name `xml:"dbmodel"`
	Roles   []struct {
		Name        string `xml:"name,attr"`
		SQLDisabled bool   `xml:"sql-disabled,attr"`
	} `xml:"role"`
	Databases []struct {
		Name        string `xml:"name,attr"`
		SQLDisabled bool   `xml:"sql-disabled,attr"`
	} `xml:"database"`
}
