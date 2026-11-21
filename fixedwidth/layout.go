package fixedwidth

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type format uint8

const (
	fmtNum format = iota + 1
	fmtAlfa
	fmtRaw
	fmtDate
	fmtMoney
	fmtConst
)

var formatNames = map[format]string{
	fmtNum: "num", fmtAlfa: "alfa", fmtRaw: "raw", fmtDate: "date", fmtMoney: "money", fmtConst: "const",
}

func (f format) String() string { return formatNames[f] }

// field is one compiled struct field.
type field struct {
	name   string
	index  []int
	start  int // 1-based, inclusive
	end    int // 1-based, inclusive
	format format
	konst  string
	kind   reflect.Kind
}

func (f *field) width() int { return f.end - f.start + 1 }

// layout is the compiled form of a struct type.
type layout struct {
	typ    reflect.Type
	fields []field // sorted by start
	width  int     // last covered position
	err    error
}

var (
	cache    sync.Map // reflect.Type -> *layout
	timeType = reflect.TypeOf(time.Time{})
)

// maxDigits is the widest num/money field each integer kind can hold
// without overflow for every digit combination.
func maxDigits(k reflect.Kind) int {
	switch k {
	case reflect.Int8, reflect.Uint8:
		return 2
	case reflect.Int16, reflect.Uint16:
		return 4
	case reflect.Int32, reflect.Uint32:
		return 9
	default: // int, int64, uint, uint64, uintptr
		return 18
	}
}

func isInt(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	}
	return false
}

func layoutOf(t reflect.Type) *layout {
	if l, ok := cache.Load(t); ok {
		return l.(*layout)
	}
	l := compile(t)
	actual, _ := cache.LoadOrStore(t, l)
	return actual.(*layout)
}

func compile(t reflect.Type) *layout {
	l := &layout{typ: t}
	var problems []string
	collect(t, nil, "", &l.fields, &problems)
	sort.SliceStable(l.fields, func(i, j int) bool { return l.fields[i].start < l.fields[j].start })
	if len(l.fields) == 0 && len(problems) == 0 {
		problems = append(problems, "no cnab-tagged fields")
	}
	next := 1
	var prev *field
	for i := range l.fields {
		f := &l.fields[i]
		switch {
		case f.start > next && prev == nil:
			problems = append(problems, fmt.Sprintf("gap at %d-%d before %s", next, f.start-1, f.name))
		case f.start > next:
			problems = append(problems, fmt.Sprintf("gap at %d-%d between %s and %s", next, f.start-1, prev.name, f.name))
		case f.start < next && prev != nil:
			problems = append(problems, fmt.Sprintf("overlap: %s (%d-%d) and %s (%d-%d)", prev.name, prev.start, prev.end, f.name, f.start, f.end))
		}
		if prev == nil || f.end > prev.end {
			prev = f
		}
		if f.end+1 > next {
			next = f.end + 1
		}
	}
	l.width = next - 1
	if len(problems) > 0 {
		l.err = &LayoutError{Type: t.String(), Problems: problems}
	}
	return l
}

func collect(t reflect.Type, index []int, prefix string, out *[]field, problems *[]string) {
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		idx := append(append([]int(nil), index...), i)
		tag, tagged := sf.Tag.Lookup("cnab")
		if tag == "-" {
			continue
		}
		if !tagged {
			if sf.Anonymous && sf.Type.Kind() == reflect.Struct && sf.Type != timeType {
				collect(sf.Type, idx, prefix, out, problems)
			}
			continue
		}
		name := prefix + sf.Name
		if !sf.IsExported() {
			*problems = append(*problems, fmt.Sprintf("%s: tagged field must be exported", name))
			continue
		}
		f, err := parseTag(name, tag, sf.Type)
		if err != "" {
			*problems = append(*problems, fmt.Sprintf("%s: %s", name, err))
			continue
		}
		f.index = idx
		*out = append(*out, f)
	}
}

func parseTag(name, tag string, t reflect.Type) (field, string) {
	f := field{name: name, kind: t.Kind()}
	parts := strings.Split(tag, ",")
	var fmtPart string
	if a, b, ok := strings.Cut(parts[0], "-"); ok {
		s, err1 := strconv.Atoi(a)
		e, err2 := strconv.Atoi(b)
		if err1 != nil || err2 != nil || len(parts) != 2 {
			return f, fmt.Sprintf("malformed tag %q, want \"start-end,format\"", tag)
		}
		f.start, f.end, fmtPart = s, e, parts[1]
	} else {
		if len(parts) != 3 {
			return f, fmt.Sprintf("malformed tag %q, want \"start,length,format\"", tag)
		}
		s, err1 := strconv.Atoi(parts[0])
		n, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return f, fmt.Sprintf("malformed tag %q, want \"start,length,format\"", tag)
		}
		if n < 1 {
			return f, fmt.Sprintf("length %d must be positive", n)
		}
		f.start, f.end, fmtPart = s, s+n-1, parts[2]
	}
	if f.start < 1 {
		return f, fmt.Sprintf("start %d out of range (positions are 1-based)", f.start)
	}
	if f.end < f.start {
		return f, fmt.Sprintf("end %d before start %d", f.end, f.start)
	}
	w := f.width()
	switch {
	case fmtPart == "num":
		f.format = fmtNum
		if t.Kind() != reflect.String && !isInt(t.Kind()) {
			return f, fmt.Sprintf("num needs a string or integer field, have %s", t)
		}
		if isInt(t.Kind()) && w > maxDigits(t.Kind()) {
			return f, fmt.Sprintf("num width %d too wide for %s (max %d); use string", w, t, maxDigits(t.Kind()))
		}
	case fmtPart == "money":
		f.format = fmtMoney
		if !isInt(t.Kind()) {
			return f, fmt.Sprintf("money needs an integer field (cents), have %s", t)
		}
		if w > maxDigits(t.Kind()) {
			return f, fmt.Sprintf("money width %d too wide for %s (max %d)", w, t, maxDigits(t.Kind()))
		}
	case fmtPart == "alfa" || fmtPart == "raw":
		f.format = fmtAlfa
		if fmtPart == "raw" {
			f.format = fmtRaw
		}
		if t.Kind() != reflect.String {
			return f, fmt.Sprintf("%s needs a string field, have %s", fmtPart, t)
		}
	case fmtPart == "date":
		f.format = fmtDate
		if t != timeType {
			return f, fmt.Sprintf("date needs a time.Time field, have %s", t)
		}
		if w != 8 && w != 6 {
			return f, fmt.Sprintf("date width must be 8 (DDMMYYYY) or 6 (DDMMYY), have %d", w)
		}
	case strings.HasPrefix(fmtPart, "const="):
		f.format = fmtConst
		f.konst = strings.TrimPrefix(fmtPart, "const=")
		if t.Kind() != reflect.String {
			return f, fmt.Sprintf("const needs a string field, have %s", t)
		}
		if len(f.konst) != w {
			return f, fmt.Sprintf("const %q has length %d, field width is %d", f.konst, len(f.konst), w)
		}
	default:
		return f, fmt.Sprintf("unknown format %q", fmtPart)
	}
	return f, ""
}

// structType returns the struct type behind v (a struct or pointer to
// struct).
func structType(v any) (reflect.Type, bool) {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil, false
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t, t.Kind() == reflect.Struct
}

// Check compiles the layout of v's type (a struct or pointer to struct) and
// verifies that it covers exactly positions 1 to lineLen. It returns a
// [*LayoutError] describing every problem found, or nil.
//
// Call it from a unit test for each record struct you define.
func Check(v any, lineLen int) error {
	t, ok := structType(v)
	if !ok {
		return ErrTarget
	}
	l := layoutOf(t)
	if l.err != nil {
		return l.err
	}
	return l.checkWidth(lineLen)
}

func (l *layout) checkWidth(lineLen int) error {
	if l.width == lineLen {
		return nil
	}
	var problems []string
	if l.width < lineLen {
		problems = append(problems, fmt.Sprintf("positions %d-%d not covered", l.width+1, lineLen))
	} else {
		for _, f := range l.fields {
			if f.end > lineLen {
				problems = append(problems, fmt.Sprintf("%s (%d-%d) exceeds line length %d", f.name, f.start, f.end, lineLen))
			}
		}
	}
	return &LayoutError{Type: l.typ.String(), Problems: problems}
}

// Width returns the record length declared by v's layout (the last covered
// position), or an error if the layout is invalid.
func Width(v any) (int, error) {
	t, ok := structType(v)
	if !ok {
		return 0, ErrTarget
	}
	l := layoutOf(t)
	if l.err != nil {
		return 0, l.err
	}
	return l.width, nil
}

// Field describes one field of a compiled layout, for documentation and
// tooling.
type Field struct {
	Name   string
	Start  int
	End    int
	Format string // num, alfa, raw, date, money or const
	Const  string // literal for const fields
}

// Fields returns the compiled fields of v's layout ordered by position.
func Fields(v any) ([]Field, error) {
	t, ok := structType(v)
	if !ok {
		return nil, ErrTarget
	}
	l := layoutOf(t)
	if l.err != nil {
		return nil, l.err
	}
	out := make([]Field, len(l.fields))
	for i, f := range l.fields {
		out[i] = Field{Name: f.name, Start: f.start, End: f.end, Format: f.format.String(), Const: f.konst}
	}
	return out, nil
}
