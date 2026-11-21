// Package file reads and writes whole CNAB files: it splits lines, detects
// the layout (240 or 400 positions), groups CNAB 240 batches and segments,
// validates trailer counts and sequence numbers, and regenerates them on
// write. It also provides a builder for CNAB 240 cobrança remessa files.
package file

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/victorcondackdev/cnab-go/cnab240"
	"github.com/victorcondackdev/cnab-go/cnab400"
	"github.com/victorcondackdev/cnab-go/fixedwidth"
)

// Sentinel errors returned (wrapped) by this package. Use [errors.Is].
var (
	// ErrEmpty reports an input with no records.
	ErrEmpty = errors.New("cnab: empty file")
	// ErrFormat reports a line length other than 240 or 400, or a file whose
	// layout does not match the function called.
	ErrFormat = errors.New("cnab: unsupported or mixed record length")
	// ErrStructure reports records out of order (missing header or trailer,
	// segment outside a batch, unexpected record type).
	ErrStructure = errors.New("cnab: invalid file structure")
	// ErrCount reports a trailer count or total that does not match the
	// records.
	ErrCount = errors.New("cnab: trailer count mismatch")
	// ErrSequence reports a batch or record sequence number out of order.
	ErrSequence = errors.New("cnab: sequence number mismatch")
	// ErrBank reports a record whose bank code differs from the header's.
	ErrBank = errors.New("cnab: bank code mismatch")
)

// LineError locates a structural problem. Err wraps one of the sentinel
// errors. Field-level problems are reported as [*fixedwidth.FieldError]
// with its Line set instead.
type LineError struct {
	Line int
	Err  error
}

// Error implements the error interface.
func (e *LineError) Error() string { return fmt.Sprintf("cnab: line %d: %v", e.Line, e.Err) }

// Unwrap returns the underlying error.
func (e *LineError) Unwrap() error { return e.Err }

func lineErr(n int, sentinel error, format string, args ...any) error {
	return &LineError{Line: n, Err: fmt.Errorf("%w: "+format, append([]any{sentinel}, args...)...)}
}

func withLine(err error, n int) error {
	var fe *fixedwidth.FieldError
	if errors.As(err, &fe) {
		fe.Line = n
		return fe
	}
	return &LineError{Line: n, Err: err}
}

// Format identifies a CNAB layout family.
type Format int

// Layout families.
const (
	FormatUnknown Format = 0
	Format240     Format = 240
	Format400     Format = 400
)

// String implements fmt.Stringer.
func (f Format) String() string {
	switch f {
	case Format240:
		return "CNAB 240"
	case Format400:
		return "CNAB 400"
	}
	return "unknown"
}

// Detect reports the layout of data from the length of its first line.
func Detect(data []byte) Format {
	line := data
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		line = data[:i]
	}
	switch len(bytes.TrimSuffix(line, []byte("\r"))) {
	case 240:
		return Format240
	case 400:
		return Format400
	}
	return FormatUnknown
}

// readLines reads r line by line, accepting LF or CRLF terminators, an
// optional final terminator and a trailing DOS end-of-file marker (0x1A).
func readLines(r io.Reader) ([]string, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	var lines []string
	for sc.Scan() {
		line := bytes.TrimSuffix(sc.Bytes(), []byte("\r"))
		lines = append(lines, string(line))
	}
	if err := sc.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, lineErr(len(lines)+1, ErrFormat, "line longer than 1 MiB")
		}
		return nil, err
	}
	for n := len(lines); n > 0; n = len(lines) {
		last := strings.TrimRight(lines[n-1], "\x1a")
		if last != "" {
			lines[n-1] = last
			break
		}
		if last == lines[n-1] { // an empty line that was not an EOF marker
			break
		}
		lines = lines[:n-1]
	}
	return lines, nil
}

// digitsEqual reports whether s is n written with exactly len(s) digits.
func digitsEqual(s string, n int) bool {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != byte('0'+n%10) {
			return false
		}
		n /= 10
	}
	return n == 0
}

// Options configures parsing. The zero value is the default used by
// [Parse].
type Options struct {
	// CheckTotals also verifies the batch trailer title quantities and
	// amounts (positions 24-115) against the batch's P or T segments when
	// the trailer reports them (any non-zero total). Off by default because
	// several banks report portfolio-wide totals there in retorno files.
	CheckTotals bool
}

// Parse reads a CNAB 240 file with the default [Options].
func Parse(r io.Reader) (*cnab240.File, error) { return Options{}.Parse(r) }

// Parse reads a CNAB 240 file and validates its structure:
//
//   - every line is 240 bytes (LF or CRLF terminated, final terminator
//     optional) and has the header's bank code;
//   - the file starts with a header (lote 0000) and ends with a trailer
//     (lote 9999), and batches are numbered 1, 2, ... in order;
//   - every record of a batch carries the batch number, detail records are
//     numbered 1, 2, ... and the batch trailer counts its records;
//   - the file trailer counts batches and lines.
//
// A 400-position input fails with [ErrFormat]; use [Parse400].
func (o Options) Parse(r io.Reader) (*cnab240.File, error) {
	lines, err := readLines(r)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, ErrEmpty
	}
	for i, l := range lines {
		if len(l) != cnab240.LineLength {
			if i == 0 && len(l) == cnab400.LineLength {
				return nil, lineErr(1, ErrFormat, "this is a CNAB 400 file; use Parse400")
			}
			return nil, lineErr(i+1, ErrFormat, "line has %d bytes, want 240", len(l))
		}
	}
	f := new(cnab240.File)
	if lines[0][7] != cnab240.RegistroHeaderArquivo {
		return nil, lineErr(1, ErrStructure, "first record must be a file header (type 0), have type %q", lines[0][7])
	}
	if err := fixedwidth.Unmarshal(lines[0], &f.Header); err != nil {
		return nil, withLine(err, 1)
	}
	if f.Header.Lote != 0 {
		return nil, lineErr(1, ErrSequence, "file header lote is %04d, want 0000", f.Header.Lote)
	}
	bank := lines[0][:3]
	var batch *cnab240.Batch
	var detailSeq int
	last := len(lines) - 1
	for i := 1; i < len(lines); i++ {
		n, l := i+1, lines[i]
		if l[:3] != bank {
			return nil, lineErr(n, ErrBank, "bank %s, file header has %s", l[:3], bank)
		}
		switch typ := l[7]; typ {
		case cnab240.RegistroHeaderLote:
			if batch != nil {
				return nil, lineErr(n, ErrStructure, "batch header inside batch %d (missing trailer)", len(f.Batches))
			}
			batch = new(cnab240.Batch)
			if err := fixedwidth.Unmarshal(l, &batch.Header); err != nil {
				return nil, withLine(err, n)
			}
			f.Batches = append(f.Batches, batch)
			if batch.Header.Lote != len(f.Batches) {
				return nil, lineErr(n, ErrSequence, "batch number %d, want %d", batch.Header.Lote, len(f.Batches))
			}
			detailSeq = 0
		case cnab240.RegistroIniciaisLote, cnab240.RegistroDetalhe, cnab240.RegistroFinaisLote:
			if batch == nil {
				return nil, lineErr(n, ErrStructure, "record type %c outside a batch", typ)
			}
			if err := checkLote(l, batch.Header.Lote, n); err != nil {
				return nil, err
			}
			if typ == cnab240.RegistroDetalhe {
				detailSeq++
				if got := l[8:13]; !digitsEqual(got, detailSeq) {
					return nil, lineErr(n, ErrSequence, "record sequence %q, want %05d", got, detailSeq)
				}
			}
			s, err := cnab240.DecodeSegment(l)
			if err != nil {
				return nil, withLine(err, n)
			}
			batch.Segments = append(batch.Segments, s)
		case cnab240.RegistroTrailerLote:
			if batch == nil {
				return nil, lineErr(n, ErrStructure, "batch trailer outside a batch")
			}
			if err := fixedwidth.Unmarshal(l, &batch.Trailer); err != nil {
				return nil, withLine(err, n)
			}
			if batch.Trailer.Lote != batch.Header.Lote {
				return nil, lineErr(n, ErrSequence, "batch trailer lote %04d, batch is %04d", batch.Trailer.Lote, batch.Header.Lote)
			}
			if want := len(batch.Segments) + 2; batch.Trailer.QuantidadeRegistros != want {
				return nil, lineErr(n, ErrCount, "batch %d trailer reports %d records, counted %d", batch.Header.Lote, batch.Trailer.QuantidadeRegistros, want)
			}
			if o.CheckTotals {
				if err := checkTotals(batch, n); err != nil {
					return nil, err
				}
			}
			batch = nil
		case cnab240.RegistroTrailerArquivo:
			if i != last {
				return nil, lineErr(n, ErrStructure, "file trailer is not the last record")
			}
			if batch != nil {
				return nil, lineErr(n, ErrStructure, "batch %d has no trailer", batch.Header.Lote)
			}
			if err := fixedwidth.Unmarshal(l, &f.Trailer); err != nil {
				return nil, withLine(err, n)
			}
			if f.Trailer.Lote != 9999 {
				return nil, lineErr(n, ErrSequence, "file trailer lote %04d, want 9999", f.Trailer.Lote)
			}
			if f.Trailer.QuantidadeLotes != len(f.Batches) {
				return nil, lineErr(n, ErrCount, "file trailer reports %d batches, counted %d", f.Trailer.QuantidadeLotes, len(f.Batches))
			}
			if f.Trailer.QuantidadeRegistros != len(lines) {
				return nil, lineErr(n, ErrCount, "file trailer reports %d records, counted %d", f.Trailer.QuantidadeRegistros, len(lines))
			}
			return f, nil
		default:
			return nil, lineErr(n, ErrStructure, "unexpected record type %q", typ)
		}
	}
	return nil, lineErr(len(lines), ErrStructure, "missing file trailer (type 9)")
}

func checkLote(line string, lote, n int) error {
	if got := line[3:7]; !digitsEqual(got, lote) {
		return lineErr(n, ErrSequence, "record lote %q, batch is %04d", got, lote)
	}
	return nil
}

func checkTotals(b *cnab240.Batch, n int) error {
	t := b.Trailer
	qty := t.QuantidadeSimples + t.QuantidadeVinculada + t.QuantidadeCaucionada + t.QuantidadeDescontada
	sum := t.ValorSimples + t.ValorVinculada + t.ValorCaucionada + t.ValorDescontada
	if qty == 0 && sum == 0 {
		return nil
	}
	var titles int
	var total int64
	for _, s := range b.Segments {
		switch s := s.(type) {
		case *cnab240.SegmentP:
			titles++
			total += s.Valor
		case *cnab240.SegmentT:
			titles++
			total += s.Valor
		}
	}
	if qty != titles {
		return lineErr(n, ErrCount, "batch %d trailer reports %d titles, counted %d", b.Header.Lote, qty, titles)
	}
	if sum != total {
		return lineErr(n, ErrCount, "batch %d trailer reports total %d cents, summed %d", b.Header.Lote, sum, total)
	}
	return nil
}

// Parse400 reads a CNAB 400 file: a header (type 0), detail records and a
// trailer (type 9), with sequence numbers (395-400) 1, 2, ... in order.
// Details are kept generic; decode them with a bank-specific struct.
func Parse400(r io.Reader) (*cnab400.File, error) {
	lines, err := readLines(r)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, ErrEmpty
	}
	for i, l := range lines {
		if len(l) != cnab400.LineLength {
			if i == 0 && len(l) == cnab240.LineLength {
				return nil, lineErr(1, ErrFormat, "this is a CNAB 240 file; use Parse")
			}
			return nil, lineErr(i+1, ErrFormat, "line has %d bytes, want 400", len(l))
		}
		if got := l[394:]; !digitsEqual(got, i+1) {
			return nil, lineErr(i+1, ErrSequence, "sequence %q, want %06d", got, i+1)
		}
	}
	if len(lines) < 2 {
		return nil, lineErr(1, ErrStructure, "a CNAB 400 file needs at least a header and a trailer")
	}
	f := &cnab400.File{Details: make([]cnab400.Detail, 0, len(lines)-2)}
	if lines[0][0] != '0' {
		return nil, lineErr(1, ErrStructure, "first record must be a header (type 0)")
	}
	if err := fixedwidth.Unmarshal(lines[0], &f.Header); err != nil {
		return nil, withLine(err, 1)
	}
	last := len(lines) - 1
	if lines[last][0] != '9' {
		return nil, lineErr(last+1, ErrStructure, "last record must be a trailer (type 9)")
	}
	if err := fixedwidth.Unmarshal(lines[last], &f.Trailer); err != nil {
		return nil, withLine(err, last+1)
	}
	for i := 1; i < last; i++ {
		if c := lines[i][0]; c == '0' || c == '9' {
			return nil, lineErr(i+1, ErrStructure, "record type %c in the middle of the file", c)
		}
		var d cnab400.Detail
		if err := fixedwidth.Unmarshal(lines[i], &d); err != nil {
			return nil, withLine(err, i+1)
		}
		f.Details = append(f.Details, d)
	}
	return f, nil
}
