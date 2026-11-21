package fixedwidth

import (
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Marshal encodes the struct v (or pointer to struct) as a single record of
// exactly lineLen bytes, without a line terminator.
//
// The layout must cover positions 1 to lineLen; otherwise a [*LayoutError]
// is returned. Value problems are reported as a [*FieldError].
func Marshal(v any, lineLen int) (string, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return "", ErrTarget
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return "", ErrTarget
	}
	l := layoutOf(rv.Type())
	if l.err != nil {
		return "", l.err
	}
	if err := l.checkWidth(lineLen); err != nil {
		return "", err
	}
	buf := make([]byte, 0, lineLen)
	for i := range l.fields {
		f := &l.fields[i]
		var err error
		if buf, err = appendField(buf, f, rv.FieldByIndex(f.index)); err != nil {
			return "", err
		}
	}
	return string(buf), nil
}

// Unmarshal decodes a single record into the struct pointed to by v. The
// line must have exactly the layout's length and must not include a line
// terminator.
//
// On error v may be partially filled. Value problems are reported as a
// [*FieldError] (with Line zero).
func Unmarshal(line string, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return ErrTarget
	}
	rv = rv.Elem()
	l := layoutOf(rv.Type())
	if l.err != nil {
		return l.err
	}
	if len(line) != l.width {
		return &FieldError{
			Field: l.typ.Name(), Start: 1, End: l.width,
			Reason: "line has " + strconv.Itoa(len(line)) + " bytes, want " + strconv.Itoa(l.width),
			Err:    ErrLineLength,
		}
	}
	for i := range l.fields {
		f := &l.fields[i]
		if err := decodeField(f, line[f.start-1:f.end], rv.FieldByIndex(f.index)); err != nil {
			return err
		}
	}
	return nil
}

func (f *field) errorf(value string, sentinel error, reason string) *FieldError {
	return &FieldError{Field: f.name, Start: f.start, End: f.end, Value: value, Reason: reason, Err: sentinel}
}

func pad(buf []byte, c byte, n int) []byte {
	for ; n > 0; n-- {
		buf = append(buf, c)
	}
	return buf
}

// appendField appends the encoding of v to buf.
func appendField(buf []byte, f *field, v reflect.Value) ([]byte, error) {
	w := f.width()
	switch f.format {
	case fmtConst:
		if s := v.String(); s != "" && s != f.konst {
			return nil, f.errorf(s, ErrConst, "must be empty or "+strconv.Quote(f.konst))
		}
		return append(buf, f.konst...), nil
	case fmtAlfa, fmtRaw:
		s := v.String()
		if f.format == fmtAlfa {
			folded, err := fold(s)
			if err != nil {
				return nil, f.errorf(s, ErrCharset, err.Error())
			}
			s = folded
		} else if strings.ContainsAny(s, "\r\n") {
			return nil, f.errorf(s, ErrCharset, "line terminator inside field")
		}
		if len(s) > w {
			return nil, f.errorf(s, ErrOverflow, "longer than "+strconv.Itoa(w)+" bytes")
		}
		return pad(append(buf, s...), ' ', w-len(s)), nil
	case fmtNum, fmtMoney:
		if v.Kind() == reflect.String {
			digits := v.String()
			for i := 0; i < len(digits); i++ {
				if digits[i] < '0' || digits[i] > '9' {
					return nil, f.errorf(digits, ErrDigits, "must contain only digits")
				}
			}
			if len(digits) > w {
				return nil, f.errorf(digits, ErrOverflow, "more than "+strconv.Itoa(w)+" digits")
			}
			return append(pad(buf, '0', w-len(digits)), digits...), nil
		}
		var u uint64
		if v.CanInt() {
			n := v.Int()
			if n < 0 {
				return nil, f.errorf(strconv.FormatInt(n, 10), ErrNegative, "negative values are not representable")
			}
			u = uint64(n)
		} else {
			u = v.Uint()
		}
		var tmp [20]byte
		digits := strconv.AppendUint(tmp[:0], u, 10)
		if len(digits) > w {
			return nil, f.errorf(string(digits), ErrOverflow, "more than "+strconv.Itoa(w)+" digits")
		}
		return append(pad(buf, '0', w-len(digits)), digits...), nil
	case fmtDate:
		t := v.Interface().(time.Time)
		if t.IsZero() {
			return pad(buf, '0', w), nil
		}
		y := t.Year()
		if w == 8 {
			if y < 1 || y > 9999 {
				return nil, f.errorf(t.Format(time.DateOnly), ErrDate, "year out of range")
			}
			return t.AppendFormat(buf, "02012006"), nil
		}
		if y < 1969 || y > 2068 {
			return nil, f.errorf(t.Format(time.DateOnly), ErrDate, "year not representable as DDMMYY (1969-2068)")
		}
		return t.AppendFormat(buf, "020106"), nil
	}
	panic("fixedwidth: unreachable format")
}

func allByte(s string, c byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != c {
			return false
		}
	}
	return true
}

func decodeField(f *field, s string, v reflect.Value) error {
	switch f.format {
	case fmtConst:
		if s != f.konst {
			return f.errorf(s, ErrConst, "expected constant "+strconv.Quote(f.konst))
		}
		v.SetString(s)
	case fmtAlfa:
		v.SetString(strings.TrimRight(s, " "))
	case fmtRaw:
		v.SetString(s)
	case fmtNum, fmtMoney:
		if allByte(s, ' ') {
			if v.Kind() == reflect.String {
				v.SetString("")
			} else {
				v.SetZero()
			}
			return nil
		}
		for i := 0; i < len(s); i++ {
			if s[i] < '0' || s[i] > '9' {
				return f.errorf(s, ErrDigits, "must contain only digits")
			}
		}
		switch {
		case v.Kind() == reflect.String:
			v.SetString(s)
		case v.CanInt():
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil || v.OverflowInt(n) {
				return f.errorf(s, ErrOverflow, "does not fit "+v.Type().String())
			}
			v.SetInt(n)
		default:
			n, err := strconv.ParseUint(s, 10, 64)
			if err != nil || v.OverflowUint(n) {
				return f.errorf(s, ErrOverflow, "does not fit "+v.Type().String())
			}
			v.SetUint(n)
		}
	case fmtDate:
		dst := v.Addr().Interface().(*time.Time)
		if allByte(s, '0') || allByte(s, ' ') {
			*dst = time.Time{}
			return nil
		}
		for i := 0; i < len(s); i++ {
			if s[i] < '0' || s[i] > '9' {
				return f.errorf(s, ErrDate, "must contain only digits")
			}
		}
		layout := "02012006"
		if len(s) == 6 {
			layout = "020106"
		}
		t, err := time.Parse(layout, s)
		if err != nil {
			return f.errorf(s, ErrDate, "not a valid date")
		}
		*dst = t
	}
	return nil
}
