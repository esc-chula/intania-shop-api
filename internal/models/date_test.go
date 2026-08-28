package models

import (
	"encoding/json"
	"testing"
)

func TestDateJSONUsesDateOnlyFormat(t *testing.T) {
	t.Parallel()

	type payload struct {
		Date Date `json:"date"`
	}

	var got payload
	if err := json.Unmarshal([]byte(`{"date":"2026-08-27"}`), &got); err != nil {
		t.Fatalf("unmarshal date: %v", err)
	}

	if got.Date.String() != "2026-08-27" {
		t.Errorf("date = %q, want %q", got.Date.String(), "2026-08-27")
	}

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal date: %v", err)
	}

	if string(encoded) != `{"date":"2026-08-27"}` {
		t.Errorf("encoded date = %s, want %s", encoded, `{"date":"2026-08-27"}`)
	}
}

func TestParseDateRejectsNonDateInput(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"2026-08-27T00:00:00Z",
		"2026-8-27",
		"not-a-date",
	} {
		if _, err := ParseDate(value); err == nil {
			t.Errorf("ParseDate(%q) succeeded, want an error", value)
		}
	}
}
