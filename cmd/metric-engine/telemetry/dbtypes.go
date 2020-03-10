package telemetry

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"time"

	"golang.org/x/xerrors"
)

// TODO:
// Most of this file, especially the JSON marshal/unmarshal are
// excellent candidates for go-based dbtypes_test.go

// Scratch is a helper type to serialize data into database string (instead of having to break out yet another table)
type Scratch struct {
	Numvalues int
	Sum       float64
	Maximum   float64
	Minimum   float64
}

// Value implements the value interface to marshal to sql
func (m Scratch) Value() (driver.Value, error) {
	b, err := json.Marshal(m)
	return b, err
}

// Scan implements the scan interface to unmarshall from sql
func (m *Scratch) Scan(src interface{}) error {
	err := json.Unmarshal(src.([]byte), m)
	if err != nil {
		return xerrors.Errorf("error parsing value from database: %w", err)
	}
	return nil
}

type RedfishDuration time.Duration

func (m RedfishDuration) Duration() time.Duration {
	return (time.Duration)(m)
}

func (m RedfishDuration) MarshalJSON() ([]byte, error) {
	return []byte((time.Duration)(m).String()), nil
}

// if this changes, the idx in conversion array below must match
const durationReString = `-?P((\d+)D)?(T((\d+)H)?((\d+)M)?(((\d+)([.]\d+)?)?S)?)?`

var durationRe = regexp.MustCompile(durationReString)

type conversion struct {
	idx        int // index into the match return from FindAllSubmatch, given above regexp
	multiplier time.Duration
}

var conversions []conversion = []conversion{
	{2, 24 * time.Hour},
	{5, time.Hour},
	{7, time.Minute},
	{10, time.Second},
}

func (m *RedfishDuration) UnmarshalJSON(data []byte) error {
	matched := durationRe.FindAllSubmatch(data, -1)
	*m += (RedfishDuration)(time.Duration(0))
	if len(matched) == 0 {
		return errors.New("could not parse Redfish Duration supplied")
	}
	for _, cnv := range conversions {
		if len(matched[0][cnv.idx]) > 0 {
			timeunits, _ := strconv.Atoi(string(matched[0][cnv.idx]))
			// fmt.Printf("Adding %v * %v\n", timeunits, cnv.multiplier)
			*m += (RedfishDuration)(time.Duration(timeunits) * cnv.multiplier)
		}
	}
	//fmt.Printf("MATCH: %q\nDURATION: %v\n\n", matched, (time.Duration)(*m))
	return nil
}

// StringArray is a type specifically to help marshal data into a single json string going into the database
type StringArray []string

// Value is the sql marshaller
func (m StringArray) Value() (driver.Value, error) {
	b, err := json.Marshal(m)
	return b, err
}

// Scan is the sql unmarshaller
func (m *StringArray) Scan(src interface{}) error {
	return json.Unmarshal(src.([]byte), m)
}
