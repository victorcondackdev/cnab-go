// Package money formats and parses amounts held as int64 cents, without
// floating point.
package money

import (
	"errors"
	"strconv"
	"strings"
)

// ErrSyntax reports an amount that cannot be parsed.
var ErrSyntax = errors.New("money: invalid amount")

// Format renders cents as "1234.56" (no thousands separator), with a
// leading "-" for negative values.
func Format(cents int64) string {
	neg := cents < 0
	u := uint64(cents)
	if neg {
		u = -u
	}
	s := strconv.FormatUint(u/100, 10) + "." + pad2(u%100)
	if neg {
		return "-" + s
	}
	return s
}

// FormatBRL renders cents as "R$ 1.234,56".
func FormatBRL(cents int64) string {
	neg := cents < 0
	u := uint64(cents)
	if neg {
		u = -u
	}
	ip := strconv.FormatUint(u/100, 10)
	var b strings.Builder
	for i, c := range ip {
		if i > 0 && (len(ip)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	s := "R$ " + b.String() + "," + pad2(u%100)
	if neg {
		return "-" + s
	}
	return s
}

func pad2(n uint64) string {
	if n < 10 {
		return "0" + strconv.FormatUint(n, 10)
	}
	return strconv.FormatUint(n, 10)
}

// Parse reads a non-negative decimal amount into cents. It accepts "1234",
// "1234.5", "1234.56", "1234,56", "1.234,56" and "1,234.56": the last '.'
// or ',' followed by one or two digits is the decimal separator and other
// separators are grouping.
func Parse(s string) (int64, error) {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "R$"))
	if s == "" {
		return 0, ErrSyntax
	}
	intPart, frac := s, ""
	if i := strings.LastIndexAny(s, ".,"); i >= 0 && len(s)-i-1 <= 2 {
		intPart, frac = s[:i], s[i+1:]
		if frac == "" {
			return 0, ErrSyntax
		}
	}
	intPart = strings.NewReplacer(".", "", ",", "").Replace(intPart)
	if intPart == "" {
		intPart = "0"
	}
	for _, p := range []string{intPart, frac} {
		for i := 0; i < len(p); i++ {
			if p[i] < '0' || p[i] > '9' {
				return 0, ErrSyntax
			}
		}
	}
	if len(frac) == 1 {
		frac += "0"
	}
	if frac == "" {
		frac = "00"
	}
	if len(intPart) > 16 {
		return 0, ErrSyntax
	}
	n, err := strconv.ParseInt(intPart+frac, 10, 64)
	if err != nil {
		return 0, ErrSyntax
	}
	return n, nil
}
