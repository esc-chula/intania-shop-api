package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

const satangPerBaht int64 = 100

// THBAmount is a non-negative Thai baht amount stored exactly as satang.
//
// The representation deliberately keeps the satang value private. Values can
// therefore only enter the domain through the checked parser or constructor,
// which keeps JSON and arithmetic operations from silently accepting invalid
// money.
type THBAmount struct {
	satang int64
}

var (
	// ErrInvalidTHBAmount reports a malformed or otherwise invalid amount.
	ErrInvalidTHBAmount = errors.New("invalid THB amount")
	// ErrTHBAmountOverflow reports an amount or arithmetic result that does not
	// fit in the fixed-point representation.
	ErrTHBAmountOverflow = errors.New("THB amount overflow")
	// ErrTHBAmountUnderflow reports a subtraction that would produce a negative
	// amount.
	ErrTHBAmountUnderflow = errors.New("THB amount underflow")
	// ErrTHBAmountNegative reports a negative amount or multiplier.
	ErrTHBAmountNegative = errors.New("THB amount must not be negative")
)

// ParseTHBAmount parses the canonical, non-negative THB wire format. The
// accepted grammar is an unsigned integer without leading zeroes followed by
// exactly two decimal digits, for example "0.00" or "640.00".
func ParseTHBAmount(value string) (THBAmount, error) {
	dot := strings.IndexByte(value, '.')
	if dot <= 0 || dot != len(value)-3 {
		return THBAmount{}, fmt.Errorf("%w: want an integer and exactly two decimal places", ErrInvalidTHBAmount)
	}

	whole := value[:dot]
	if len(whole) > 1 && whole[0] == '0' {
		return THBAmount{}, fmt.Errorf("%w: leading zeroes are not allowed", ErrInvalidTHBAmount)
	}
	for index := 0; index < len(whole); index++ {
		if whole[index] < '0' || whole[index] > '9' {
			return THBAmount{}, fmt.Errorf("%w: amount must contain only ASCII digits", ErrInvalidTHBAmount)
		}
	}
	for index := dot + 1; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return THBAmount{}, fmt.Errorf("%w: amount must contain only ASCII digits", ErrInvalidTHBAmount)
		}
	}

	wholeSatang, err := parseWholeSatang(whole)
	if err != nil {
		return THBAmount{}, err
	}
	fractionSatang := int64(value[dot+1]-'0')*10 + int64(value[dot+2]-'0')
	if wholeSatang > (math.MaxInt64-fractionSatang)/satangPerBaht {
		return THBAmount{}, ErrTHBAmountOverflow
	}
	return THBAmount{satang: wholeSatang*satangPerBaht + fractionSatang}, nil
}

// NewTHBAmountFromSatang constructs an amount from an exact satang value.
func NewTHBAmountFromSatang(satang int64) (THBAmount, error) {
	if satang < 0 {
		return THBAmount{}, ErrTHBAmountNegative
	}
	return THBAmount{satang: satang}, nil
}

// Satang returns the exact amount in satang.
func (amount THBAmount) Satang() int64 { return amount.satang }

// String returns the canonical two-decimal representation.
func (amount THBAmount) String() string {
	return fmt.Sprintf("%d.%02d", amount.satang/satangPerBaht, amount.satang%satangPerBaht)
}

// MarshalJSON always emits a JSON string, never a JSON number.
func (amount THBAmount) MarshalJSON() ([]byte, error) {
	return json.Marshal(amount.String())
}

// UnmarshalJSON accepts only a JSON string in the canonical THB format.
func (amount *THBAmount) UnmarshalJSON(data []byte) error {
	if amount == nil {
		return errors.New("cannot unmarshal THB amount into a nil receiver")
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("%w: amount must be a JSON string", ErrInvalidTHBAmount)
	}
	parsed, err := ParseTHBAmount(value)
	if err != nil {
		return err
	}
	*amount = parsed
	return nil
}

// MarshalText implements encoding.TextMarshaler using the same canonical
// representation as JSON.
func (amount THBAmount) MarshalText() ([]byte, error) { return []byte(amount.String()), nil }

// UnmarshalText parses the canonical representation used by JSON strings and
// database values.
func (amount *THBAmount) UnmarshalText(text []byte) error {
	if amount == nil {
		return errors.New("cannot unmarshal THB amount into a nil receiver")
	}
	parsed, err := ParseTHBAmount(string(text))
	if err != nil {
		return err
	}
	*amount = parsed
	return nil
}

// Add returns the exact sum, rejecting fixed-point overflow.
func (amount THBAmount) Add(other THBAmount) (THBAmount, error) {
	if other.satang > math.MaxInt64-amount.satang {
		return THBAmount{}, ErrTHBAmountOverflow
	}
	return THBAmount{satang: amount.satang + other.satang}, nil
}

// Sub returns the exact difference, rejecting a negative result.
func (amount THBAmount) Sub(other THBAmount) (THBAmount, error) {
	if other.satang > amount.satang {
		return THBAmount{}, ErrTHBAmountUnderflow
	}
	return THBAmount{satang: amount.satang - other.satang}, nil
}

// Subtract is the descriptive alias for Sub.
func (amount THBAmount) Subtract(other THBAmount) (THBAmount, error) {
	return amount.Sub(other)
}

// Mul returns amount multiplied by a non-negative integer quantity, rejecting
// fixed-point overflow. A zero quantity is valid and returns zero.
func (amount THBAmount) Mul(quantity int64) (THBAmount, error) {
	if quantity < 0 {
		return THBAmount{}, ErrTHBAmountNegative
	}
	if quantity != 0 && amount.satang > math.MaxInt64/quantity {
		return THBAmount{}, ErrTHBAmountOverflow
	}
	return THBAmount{satang: amount.satang * quantity}, nil
}

// Multiply is the descriptive alias for Mul.
func (amount THBAmount) Multiply(quantity int64) (THBAmount, error) {
	return amount.Mul(quantity)
}

// Value implements database/sql/driver.Valuer so database writes retain the
// exact two-decimal representation.
func (amount THBAmount) Value() (driver.Value, error) { return amount.String(), nil }

// Scan implements database/sql.Scanner for PostgreSQL NUMERIC values returned
// as strings or byte slices. Floating-point values are intentionally rejected.
func (amount *THBAmount) Scan(value any) error {
	if amount == nil {
		return errors.New("cannot scan THB amount into a nil receiver")
	}
	var text string
	switch value := value.(type) {
	case string:
		text = value
	case []byte:
		text = string(value)
	default:
		return fmt.Errorf("%w: database value must be text", ErrInvalidTHBAmount)
	}
	return amount.UnmarshalText([]byte(text))
}

func parseWholeSatang(value string) (int64, error) {
	var whole int64
	for index := 0; index < len(value); index++ {
		digit := int64(value[index] - '0')
		if whole > (math.MaxInt64-digit)/10 {
			return 0, ErrTHBAmountOverflow
		}
		whole = whole*10 + digit
	}
	return whole, nil
}
