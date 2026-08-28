package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// Date represents a calendar date without a time of day.
type Date struct {
	time.Time
}

const dateLayout = "2006-01-02"

// NewDate returns the calendar date represented by value.
func NewDate(value time.Time) Date {
	year, month, day := value.Date()
	return Date{Time: time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

// ParseDate parses a date-only value in YYYY-MM-DD format.
func ParseDate(value string) (Date, error) {
	parsed, err := time.Parse(dateLayout, value)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q: %w", value, err)
	}
	return NewDate(parsed), nil
}

// MarshalJSON serializes a Date without a time of day or timezone.
func (date Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(date.String())
}

// UnmarshalJSON parses a date-only JSON string in YYYY-MM-DD format.
func (date *Date) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("date must be a string: %w", err)
	}

	parsed, err := ParseDate(value)
	if err != nil {
		return err
	}
	*date = parsed
	return nil
}

// String returns the date in YYYY-MM-DD format.
func (date Date) String() string {
	year, month, day := date.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Format(dateLayout)
}
