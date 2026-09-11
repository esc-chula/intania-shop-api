package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"math"
	"testing"
)

func TestParseTHBAmount(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "zero", value: "0.00", want: "0.00"},
		{name: "whole amount", value: "640.00", want: "640.00"},
		{name: "satang", value: "0.01", want: "0.01"},
		{name: "maximum", value: "92233720368547758.07", want: "92233720368547758.07"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			amount, err := ParseTHBAmount(test.value)
			if err != nil {
				t.Fatalf("ParseTHBAmount(%q): %v", test.value, err)
			}
			if got := amount.String(); got != test.want {
				t.Fatalf("String()=%q want %q", got, test.want)
			}
		})
	}
}

func TestParseTHBAmountRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{
		"",
		"1",
		"1.0",
		"1.000",
		".00",
		"00.00",
		"01.00",
		"-1.00",
		"+1.00",
		"1,000.00",
		"1e2.00",
		" 1.00",
		"1.00 ",
		"92233720368547758.08",
		"922337203685477580.00",
	} {
		if _, err := ParseTHBAmount(value); err == nil {
			t.Errorf("ParseTHBAmount(%q) succeeded; want error", value)
		}
	}
}

func TestTHBAmountJSONIsStringOnly(t *testing.T) {
	amount, err := ParseTHBAmount("640.00")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(amount)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(encoded); got != `"640.00"` {
		t.Fatalf("MarshalJSON()=%s", got)
	}

	var decoded THBAmount
	if err := json.Unmarshal([]byte(`"640.00"`), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != amount {
		t.Fatalf("decoded=%v want %v", decoded, amount)
	}
	for _, input := range []string{`640.00`, `null`, `"640"`, `"-1.00"`} {
		if err := json.Unmarshal([]byte(input), &decoded); err == nil {
			t.Errorf("json.Unmarshal(%s) succeeded; want error", input)
		}
	}
}

func TestTHBAmountArithmetic(t *testing.T) {
	left, err := ParseTHBAmount("640.25")
	if err != nil {
		t.Fatal(err)
	}
	right, err := ParseTHBAmount("12.75")
	if err != nil {
		t.Fatal(err)
	}

	sum, err := left.Add(right)
	if err != nil || sum.String() != "653.00" {
		t.Fatalf("Add()=(%s, %v)", sum.String(), err)
	}
	difference, err := left.Sub(right)
	if err != nil || difference.String() != "627.50" {
		t.Fatalf("Sub()=(%s, %v)", difference.String(), err)
	}
	product, err := right.Mul(3)
	if err != nil || product.String() != "38.25" {
		t.Fatalf("Mul()=(%s, %v)", product.String(), err)
	}
	zero, err := right.Mul(0)
	if err != nil || zero.String() != "0.00" {
		t.Fatalf("Mul(0)=(%s, %v)", zero.String(), err)
	}

	if _, err := right.Sub(left); !errors.Is(err, ErrTHBAmountUnderflow) {
		t.Fatalf("Sub underflow error=%v", err)
	}
	if _, err := right.Mul(-1); !errors.Is(err, ErrTHBAmountNegative) {
		t.Fatalf("Mul negative error=%v", err)
	}
}

func TestTHBAmountArithmeticOverflow(t *testing.T) {
	maximum, err := NewTHBAmountFromSatang(math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	oneSatang, err := NewTHBAmountFromSatang(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := maximum.Add(oneSatang); !errors.Is(err, ErrTHBAmountOverflow) {
		t.Fatalf("Add overflow error=%v", err)
	}
	if _, err := maximum.Mul(2); !errors.Is(err, ErrTHBAmountOverflow) {
		t.Fatalf("Mul overflow error=%v", err)
	}
}

func TestTHBAmountDatabaseValue(t *testing.T) {
	amount, err := ParseTHBAmount("640.00")
	if err != nil {
		t.Fatal(err)
	}
	value, err := amount.Value()
	if err != nil {
		t.Fatal(err)
	}
	if value != driver.Value("640.00") {
		t.Fatalf("Value()=%v", value)
	}
	var scanned THBAmount
	if err := scanned.Scan([]byte("640.00")); err != nil {
		t.Fatal(err)
	}
	if scanned != amount {
		t.Fatalf("scanned=%v want %v", scanned, amount)
	}
	if err := scanned.Scan(float64(640)); err == nil {
		t.Fatal("Scan(float64) succeeded; want error")
	}
}
