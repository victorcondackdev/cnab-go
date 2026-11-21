package money

import "testing"

func TestFormat(t *testing.T) {
	tests := []struct {
		in       int64
		dot, brl string
	}{
		{0, "0.00", "R$ 0,00"},
		{5, "0.05", "R$ 0,05"},
		{123456, "1234.56", "R$ 1.234,56"},
		{100000000, "1000000.00", "R$ 1.000.000,00"},
		{-1050, "-10.50", "-R$ 10,50"},
	}
	for _, tt := range tests {
		if got := Format(tt.in); got != tt.dot {
			t.Errorf("Format(%d) = %q, want %q", tt.in, got, tt.dot)
		}
		if got := FormatBRL(tt.in); got != tt.brl {
			t.Errorf("FormatBRL(%d) = %q, want %q", tt.in, got, tt.brl)
		}
	}
}

func TestParse(t *testing.T) {
	ok := map[string]int64{
		"1234": 123400, "1234.5": 123450, "1234.56": 123456, "1234,56": 123456,
		"1.234,56": 123456, "1,234.56": 123456, "R$ 10,00": 1000, ",5": 50, "0.01": 1,
		"1.234": 123400,
	}
	for in, want := range ok {
		if got, err := Parse(in); err != nil || got != want {
			t.Errorf("Parse(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "12.", "-5", "1.2.3a", "99999999999999999999"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) succeeded", in)
		}
	}
}
