package file

import (
	"bufio"
	"fmt"
	"io"

	"github.com/victorcondackdev/cnab-go/cnab240"
	"github.com/victorcondackdev/cnab-go/cnab400"
	"github.com/victorcondackdev/cnab-go/fixedwidth"
)

// Writer configures how files are written. The zero value writes CRLF line
// endings with a final terminator, which is what bank systems expect.
type Writer struct {
	// LineEnding overrides the terminator ("\r\n" when empty).
	LineEnding string
	// OmitFinalNewline drops the terminator after the last record.
	OmitFinalNewline bool
}

// Write writes f as CNAB 240 with the default [Writer].
func Write(w io.Writer, f *cnab240.File) error { return Writer{}.Write(w, f) }

// Write writes f as CNAB 240. Control fields are regenerated, so callers
// never maintain them by hand:
//
//   - every record gets the file header's bank code;
//   - batches are numbered 1, 2, ... and every record of a batch carries
//     its number; the file header and trailer get 0000 and 9999;
//   - type 3 records are numbered 1, 2, ... within their batch;
//   - batch and file trailer record counts and the batch count are set.
//
// f itself is not modified. Parse followed by Write reproduces a valid
// file byte for byte when the line endings match and every alfa field was
// already upper-case ASCII.
func (wr Writer) Write(w io.Writer, f *cnab240.File) error {
	eol := wr.LineEnding
	if eol == "" {
		eol = "\r\n"
	}
	if len(f.Batches) > 9998 {
		return fmt.Errorf("%w: %d batches, at most 9998 fit", ErrCount, len(f.Batches))
	}
	records := 2
	for i, b := range f.Batches {
		if len(b.Segments) > 99999 {
			return fmt.Errorf("%w: batch %d has %d records, at most 99999 fit", ErrCount, i+1, len(b.Segments))
		}
		records += len(b.Segments) + 2
	}
	if records > 999999 {
		return fmt.Errorf("%w: %d records, at most 999999 fit", ErrCount, records)
	}
	bw := bufio.NewWriterSize(w, 64*1024)
	total := 0
	first := true
	scratch := make([]byte, 0, cnab240.LineLength)
	emitBytes := func(line []byte) {
		if !first {
			bw.WriteString(eol)
		}
		first = false
		bw.Write(line)
		total++
	}
	emit := func(line string) {
		if !first {
			bw.WriteString(eol)
		}
		first = false
		bw.WriteString(line)
		total++
	}

	h := f.Header
	h.Lote = 0
	line, err := fixedwidth.Marshal(&h, cnab240.LineLength)
	if err != nil {
		return fmt.Errorf("file header: %w", err)
	}
	bank := line[:3]
	emit(line)
	for bi, b := range f.Batches {
		lote := bi + 1
		bh := b.Header
		bh.Banco, bh.Lote = bank, lote
		if line, err = fixedwidth.Marshal(&bh, cnab240.LineLength); err != nil {
			return fmt.Errorf("batch %d header: %w", lote, err)
		}
		emit(line)
		seq := 0
		for si, s := range b.Segments {
			if line, err = cnab240.EncodeSegment(s); err != nil {
				return fmt.Errorf("batch %d record %d: %w", lote, si+1, err)
			}
			scratch = append(scratch[:0], line...)
			copy(scratch[0:3], bank)
			putDigits(scratch[3:7], lote)
			if scratch[7] == cnab240.RegistroDetalhe {
				seq++
				putDigits(scratch[8:13], seq)
			}
			emitBytes(scratch)
		}
		bt := b.Trailer
		bt.Banco, bt.Lote = bank, lote
		bt.QuantidadeRegistros = len(b.Segments) + 2
		if line, err = fixedwidth.Marshal(&bt, cnab240.LineLength); err != nil {
			return fmt.Errorf("batch %d trailer: %w", lote, err)
		}
		emit(line)
	}
	ft := f.Trailer
	ft.Banco, ft.Lote = bank, 9999
	ft.QuantidadeLotes = len(f.Batches)
	ft.QuantidadeRegistros = total + 1
	if line, err = fixedwidth.Marshal(&ft, cnab240.LineLength); err != nil {
		return fmt.Errorf("file trailer: %w", err)
	}
	emit(line)
	if !wr.OmitFinalNewline {
		bw.WriteString(eol)
	}
	return bw.Flush()
}

// Write400 writes f as CNAB 400 with the default [Writer].
func Write400(w io.Writer, f *cnab400.File) error { return Writer{}.Write400(w, f) }

// Write400 writes f as CNAB 400, regenerating the sequence numbers
// (395-400). f is not modified.
func (wr Writer) Write400(w io.Writer, f *cnab400.File) error {
	eol := wr.LineEnding
	if eol == "" {
		eol = "\r\n"
	}
	bw := bufio.NewWriter(w)
	seq := 0
	write := func(v any) error {
		seq++
		line, err := fixedwidth.Marshal(v, cnab400.LineLength)
		if err != nil {
			return fmt.Errorf("record %d: %w", seq, err)
		}
		if seq > 1 {
			bw.WriteString(eol)
		}
		bw.WriteString(line)
		return nil
	}
	h := f.Header
	h.Sequencial = 1
	if err := write(&h); err != nil {
		return err
	}
	for _, d := range f.Details {
		d.Sequencial = seq + 1
		if err := write(&d); err != nil {
			return err
		}
	}
	t := f.Trailer
	t.Sequencial = seq + 1
	if err := write(&t); err != nil {
		return err
	}
	if !wr.OmitFinalNewline {
		bw.WriteString(eol)
	}
	return bw.Flush()
}

// putDigits writes n zero-padded into dst (n must fit).
func putDigits(dst []byte, n int) {
	for i := len(dst) - 1; i >= 0; i-- {
		dst[i] = byte('0' + n%10)
		n /= 10
	}
}
