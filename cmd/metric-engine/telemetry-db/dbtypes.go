package telemetry

import (
	"database/sql/driver"
	"encoding/json"

	"golang.org/x/xerrors"
)

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
