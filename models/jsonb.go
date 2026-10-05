package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONB is a raw JSON document stored in a jsonb column. It is written as
// text (so Postgres parses it as JSON, not bytea) and marshals as itself.
type JSONB []byte

func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

func (j *JSONB) Scan(src interface{}) error {
	switch v := src.(type) {
	case nil:
		*j = nil
	case []byte:
		*j = append(JSONB(nil), v...)
	case string:
		*j = JSONB(v)
	default:
		return fmt.Errorf("JSONB: cannot scan %T", src)
	}
	return nil
}

func (j JSONB) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

func (j *JSONB) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*j = nil
		return nil
	}
	*j = append(JSONB(nil), b...)
	return nil
}

// ToJSONB marshals v, for audit before/after values.
func ToJSONB(v interface{}) JSONB {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
