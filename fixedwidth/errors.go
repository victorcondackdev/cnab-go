package fixedwidth

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors returned (possibly wrapped) by this package. Use
// [errors.Is] to test for them.
var (
	// ErrLayout reports an invalid struct layout. The concrete error is a
	// [*LayoutError].
	ErrLayout = errors.New("fixedwidth: invalid layout")
	// ErrLineLength reports a line whose length differs from the layout's.
	ErrLineLength = errors.New("fixedwidth: wrong line length")
	// ErrDigits reports a numeric field holding something other than digits.
	ErrDigits = errors.New("fixedwidth: non-digit in numeric field")
	// ErrOverflow reports a value that does not fit its field or Go type.
	ErrOverflow = errors.New("fixedwidth: value does not fit")
	// ErrNegative reports a negative number; CNAB numeric fields are unsigned.
	ErrNegative = errors.New("fixedwidth: negative value")
	// ErrCharset reports a rune that cannot be represented in an alfa field.
	ErrCharset = errors.New("fixedwidth: unsupported character")
	// ErrDate reports an invalid or unrepresentable date.
	ErrDate = errors.New("fixedwidth: invalid date")
	// ErrConst reports a constant field that does not hold its literal.
	ErrConst = errors.New("fixedwidth: constant mismatch")
	// ErrTarget reports a nil, non-pointer or non-struct argument.
	ErrTarget = errors.New("fixedwidth: target must be a non-nil pointer to struct")
)

// FieldError locates a value that could not be encoded or decoded.
//
// Start and End are the 1-based inclusive positions of the field, as in the
// bank manuals. Line is the 1-based line number in a file; it is zero when
// the error comes straight from [Marshal] or [Unmarshal] and is filled in by
// file-level parsers. Err is one of the package's sentinel errors, so
// [errors.Is] works on a FieldError.
type FieldError struct {
	Line   int
	Field  string
	Start  int
	End    int
	Value  string
	Reason string
	Err    error
}

// Error implements the error interface.
func (e *FieldError) Error() string {
	var b strings.Builder
	b.WriteString("cnab: ")
	if e.Line > 0 {
		fmt.Fprintf(&b, "line %d: ", e.Line)
	}
	fmt.Fprintf(&b, "field %s (%d-%d)", e.Field, e.Start, e.End)
	if e.Value != "" {
		fmt.Fprintf(&b, " value %q", e.Value)
	}
	b.WriteString(": ")
	b.WriteString(e.Reason)
	return b.String()
}

// Unwrap returns the sentinel error.
func (e *FieldError) Unwrap() error { return e.Err }

// LayoutError lists every problem found while compiling a struct layout.
// It matches [ErrLayout] with [errors.Is].
type LayoutError struct {
	Type     string
	Problems []string
}

// Error implements the error interface.
func (e *LayoutError) Error() string {
	return fmt.Sprintf("fixedwidth: invalid layout %s: %s", e.Type, strings.Join(e.Problems, "; "))
}

// Is reports whether target is [ErrLayout].
func (e *LayoutError) Is(target error) bool { return target == ErrLayout }
