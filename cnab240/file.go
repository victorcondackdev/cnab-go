package cnab240

import (
	"errors"
	"fmt"

	"github.com/victorcondackdev/cnab-go/fixedwidth"
)

// File is a parsed CNAB 240 file.
type File struct {
	Header  FileHeader
	Batches []*Batch
	Trailer FileTrailer
}

// Batch is one "lote": its header, detail records in file order and
// trailer.
type Batch struct {
	Header   BatchHeader
	Segments []Segment
	Trailer  BatchTrailer
}

// Titulo groups the segments describing one title in a remessa: P plus the
// Q and R that follow it (Q and R are nil when absent).
type Titulo struct {
	P *SegmentP
	Q *SegmentQ
	R *SegmentR
}

// Ocorrencia groups the segments describing one occurrence in a retorno:
// T plus the U that follows it (nil when absent).
type Ocorrencia struct {
	T *SegmentT
	U *SegmentU
}

// Titulos groups the batch's P/Q/R segments. A Q or R not preceded by a P
// is ignored.
func (b *Batch) Titulos() []Titulo {
	var out []Titulo
	for _, s := range b.Segments {
		switch s := s.(type) {
		case *SegmentP:
			out = append(out, Titulo{P: s})
		case *SegmentQ:
			if n := len(out); n > 0 && out[n-1].Q == nil {
				out[n-1].Q = s
			}
		case *SegmentR:
			if n := len(out); n > 0 && out[n-1].R == nil {
				out[n-1].R = s
			}
		}
	}
	return out
}

// Ocorrencias groups the batch's T/U segments. A U not preceded by a T is
// ignored.
func (b *Batch) Ocorrencias() []Ocorrencia {
	var out []Ocorrencia
	for _, s := range b.Segments {
		switch s := s.(type) {
		case *SegmentT:
			out = append(out, Ocorrencia{T: s})
		case *SegmentU:
			if n := len(out); n > 0 && out[n-1].U == nil {
				out[n-1].U = s
			}
		}
	}
	return out
}

// Ocorrencias returns the occurrences of every batch in file order.
func (f *File) Ocorrencias() []Ocorrencia {
	var out []Ocorrencia
	for _, b := range f.Batches {
		out = append(out, b.Ocorrencias()...)
	}
	return out
}

// Titulos returns the titles of every batch in file order.
func (f *File) Titulos() []Titulo {
	var out []Titulo
	for _, b := range f.Batches {
		out = append(out, b.Titulos()...)
	}
	return out
}

// ErrRawLength reports a RawSegment whose line is not 240 bytes.
var ErrRawLength = errors.New("cnab240: raw segment must be 240 bytes")

// EncodeSegment encodes a detail record as a 240-byte line.
func EncodeSegment(s Segment) (string, error) {
	if r, ok := s.(*RawSegment); ok {
		if len(r.Line) != LineLength {
			return "", fmt.Errorf("%w, have %d", ErrRawLength, len(r.Line))
		}
		return r.Line, nil
	}
	return fixedwidth.Marshal(s, LineLength)
}

// DecodeSegment decodes a 240-byte batch record (type 2, 3 or 4). Type 3
// records with segment codes P, Q, R, T or U decode to their struct; any
// other record is returned as a [*RawSegment].
func DecodeSegment(line string) (Segment, error) {
	if len(line) != LineLength {
		return nil, &fixedwidth.FieldError{Field: "Segment", Start: 1, End: LineLength,
			Reason: fmt.Sprintf("line has %d bytes, want %d", len(line), LineLength), Err: fixedwidth.ErrLineLength}
	}
	var s Segment
	if line[7] == RegistroDetalhe {
		switch line[13] {
		case 'P':
			s = new(SegmentP)
		case 'Q':
			s = new(SegmentQ)
		case 'R':
			s = new(SegmentR)
		case 'T':
			s = new(SegmentT)
		case 'U':
			s = new(SegmentU)
		}
	}
	if s == nil {
		return &RawSegment{Line: line}, nil
	}
	if err := fixedwidth.Unmarshal(line, s); err != nil {
		return nil, err
	}
	return s, nil
}
