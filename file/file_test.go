package file

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/victorcondackdev/cnab-go/cnab240"
	"github.com/victorcondackdev/cnab-go/cnab400/bradesco"
	"github.com/victorcondackdev/cnab-go/fixedwidth"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func d(y, m, dd int) time.Time { return time.Date(y, time.Month(m), dd, 0, 0, 0, 0, time.UTC) }

func TestRoundTripSamples(t *testing.T) {
	for _, name := range []string{"remessa_341.rem", "retorno_341.ret"} {
		t.Run(name, func(t *testing.T) {
			data := read(t, name)
			f, err := Parse(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := Write(&buf, f); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(buf.Bytes(), data) {
				t.Fatalf("Write(Parse(%s)) differs from input", name)
			}
			// LF, no final newline: parses identically and round-trips with
			// the matching Writer.
			lf := bytes.TrimSuffix(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")), []byte("\n"))
			f2, err := Parse(bytes.NewReader(lf))
			if err != nil {
				t.Fatal(err)
			}
			buf.Reset()
			if err := (Writer{LineEnding: "\n", OmitFinalNewline: true}).Write(&buf, f2); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(buf.Bytes(), lf) {
				t.Fatal("LF round trip differs")
			}
		})
	}
}

// TestRemessaPositions checks the sample remessa at raw byte offsets taken
// from the FEBRABAN manual, independently of the struct tags.
func TestRemessaPositions(t *testing.T) {
	lines := strings.Split(strings.TrimSuffix(string(read(t, "remessa_341.rem")), "\r\n"), "\r\n")
	at := func(line, start, end int) string { return lines[line][start-1 : end] }
	checks := []struct {
		line, start, end int
		want             string
	}{
		{0, 1, 3, "341"}, {0, 4, 7, "0000"}, {0, 8, 8, "0"}, {0, 18, 18, "2"}, {0, 19, 32, "11222333000181"},
		{0, 143, 143, "1"}, {0, 144, 151, "20112026"}, {0, 152, 157, "093015"}, {0, 158, 163, "000042"},
		{1, 8, 8, "1"}, {1, 9, 9, "R"}, {1, 10, 11, "01"}, {1, 19, 33, "011222333000181"}, {1, 184, 191, "00000042"},
		{2, 8, 8, "3"}, {2, 9, 13, "00001"}, {2, 14, 14, "P"}, {2, 16, 17, "01"}, {2, 38, 45, "00000001"},
		{2, 63, 69, "NF-1001"}, {2, 78, 85, "10122026"}, {2, 86, 100, "000000000150000"}, {2, 107, 108, "02"},
		{2, 109, 109, "N"}, {2, 118, 118, "1"}, {2, 119, 126, "11122026"}, {2, 127, 141, "000000000000050"},
		{2, 228, 229, "09"},
		{3, 14, 14, "Q"}, {3, 18, 18, "1"}, {3, 19, 33, "000052998224725"}, {3, 34, 52, "JOAO DA SILVA SAURO"},
		{3, 129, 136, "22240003"}, {3, 137, 150, "RIO DE JANEIRO"}, {3, 152, 153, "RJ"},
		{6, 14, 14, "R"}, {6, 66, 66, "1"}, {6, 67, 74, "16122026"}, {6, 75, 89, "000000000001800"},
		{13, 8, 8, "5"}, {13, 18, 23, "000013"},
		{14, 4, 7, "9999"}, {14, 18, 23, "000001"}, {14, 24, 29, "000015"},
	}
	for _, c := range checks {
		if got := at(c.line, c.start, c.end); got != c.want {
			t.Errorf("line %d pos %d-%d = %q, want %q", c.line+1, c.start, c.end, got, c.want)
		}
	}
}

func TestParseRetorno(t *testing.T) {
	f, err := Options{CheckTotals: true}.Parse(bytes.NewReader(read(t, "retorno_341.ret")))
	if err != nil {
		t.Fatal(err)
	}
	if f.Header.CodigoRemessaRetorno != cnab240.CodigoRetorno || f.Header.NomeEmpresa != "ACME UTILIDADES LTDA" {
		t.Errorf("header %+v", f.Header)
	}
	occ := f.Ocorrencias()
	if len(occ) != 9 {
		t.Fatalf("got %d occurrences", len(occ))
	}
	o := occ[4]
	if o.T.NossoNumero != "00000002" || o.T.Movimento != cnab240.RetornoLiquidacao || o.U.ValorPago != 91880 ||
		o.U.JurosMulta != 1890 || !o.U.DataOcorrencia.Equal(d(2026, 12, 17)) || o.T.Tarifa != 350 {
		t.Errorf("occurrence 5: %+v %+v", o.T, o.U)
	}
	if got := occ[2].T.MotivoCodes(); len(got) != 2 || got[0] != "08" || got[1] != "46" {
		t.Errorf("motivos %v", got)
	}
}

func TestParseRemessaTitulos(t *testing.T) {
	f, err := Parse(bytes.NewReader(read(t, "remessa_341.rem")))
	if err != nil {
		t.Fatal(err)
	}
	ts := f.Titulos()
	if len(ts) != 5 || ts[1].R == nil || ts[0].R != nil || ts[1].Q.Nome != "PADARIA PAO QUENTE LTDA" {
		t.Fatalf("titulos %+v", ts)
	}
	if ts[1].R.Mensagem3 != "NAO RECEBER APOS 30 DIAS DO VENCIMENTO" {
		t.Errorf("mensagem %q", ts[1].R.Mensagem3)
	}
}

func lines240(t *testing.T) []string {
	return strings.Split(strings.TrimSuffix(string(read(t, "remessa_341.rem")), "\r\n"), "\r\n")
}

func join(ls []string) *strings.Reader { return strings.NewReader(strings.Join(ls, "\r\n") + "\r\n") }

func patch(l string, pos int, s string) string { return l[:pos-1] + s + l[pos-1+len(s):] }

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]string) []string
		err    error
		line   int
	}{
		{"short line", func(l []string) []string { l[3] = l[3][:239]; return l }, ErrFormat, 4},
		{"no header", func(l []string) []string { return l[1:] }, ErrStructure, 1},
		{"no trailer", func(l []string) []string { return l[:len(l)-1] }, ErrStructure, 14},
		{"trailer not last", func(l []string) []string { return append(l, l[len(l)-1]) }, ErrStructure, 15},
		{"bank", func(l []string) []string { l[2] = patch(l[2], 1, "001"); return l }, ErrBank, 3},
		{"header lote", func(l []string) []string { l[0] = patch(l[0], 4, "0001"); return l }, ErrSequence, 1},
		{"batch number", func(l []string) []string { l[1] = patch(l[1], 4, "0002"); return l }, ErrSequence, 2},
		{"segment lote", func(l []string) []string { l[4] = patch(l[4], 4, "0002"); return l }, ErrSequence, 5},
		{"segment seq", func(l []string) []string { l[4] = patch(l[4], 9, "00009"); return l }, ErrSequence, 5},
		{"batch count", func(l []string) []string { l[13] = patch(l[13], 18, "000099"); return l }, ErrCount, 14},
		{"batch trailer lote", func(l []string) []string { l[13] = patch(l[13], 4, "0003"); return l }, ErrSequence, 14},
		{"file lotes", func(l []string) []string { l[14] = patch(l[14], 18, "000002"); return l }, ErrCount, 15},
		{"file records", func(l []string) []string { l[14] = patch(l[14], 24, "000016"); return l }, ErrCount, 15},
		{"file trailer lote", func(l []string) []string { l[14] = patch(l[14], 4, "9998"); return l }, ErrSequence, 15},
		{"segment outside batch", func(l []string) []string {
			return append([]string{l[0], l[2]}, l[1:]...)
		}, ErrStructure, 2},
		{"trailer outside batch", func(l []string) []string {
			return append([]string{l[0], l[13]}, l[1:]...)
		}, ErrStructure, 2},
		{"nested batch", func(l []string) []string {
			return append(l[:3:3], append([]string{l[1]}, l[3:]...)...)
		}, ErrStructure, 4},
		{"unclosed batch", func(l []string) []string { return append(l[:13:13], l[14]) }, ErrStructure, 14},
		{"record type", func(l []string) []string { l[2] = patch(l[2], 8, "7"); return l }, ErrStructure, 3},
		{"field", func(l []string) []string { l[2] = patch(l[2], 86, "00000000000A000"); return l }, fixedwidth.ErrDigits, 3},
		{"header field", func(l []string) []string { l[0] = patch(l[0], 144, "32132026"); return l }, fixedwidth.ErrDate, 1},
		{"batch header field", func(l []string) []string { l[1] = patch(l[1], 10, "X1"); return l }, fixedwidth.ErrDigits, 2},
		{"batch trailer field", func(l []string) []string { l[13] = patch(l[13], 24, "00000X"); return l }, fixedwidth.ErrDigits, 14},
		{"file trailer field", func(l []string) []string { l[14] = patch(l[14], 30, "00000X"); return l }, fixedwidth.ErrDigits, 15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(join(tt.mutate(lines240(t))))
			if !errors.Is(err, tt.err) {
				t.Fatalf("got %v, want %v", err, tt.err)
			}
			var le *LineError
			var fe *fixedwidth.FieldError
			switch {
			case errors.As(err, &le):
				if le.Line != tt.line {
					t.Errorf("line %d, want %d (%v)", le.Line, tt.line, err)
				}
			case errors.As(err, &fe):
				if fe.Line != tt.line {
					t.Errorf("line %d, want %d (%v)", fe.Line, tt.line, err)
				}
			default:
				t.Errorf("error %T carries no line", err)
			}
		})
	}
	if _, err := Parse(strings.NewReader("")); !errors.Is(err, ErrEmpty) {
		t.Errorf("empty: %v", err)
	}
	if _, err := Parse(bytes.NewReader(read(t, "remessa_237.rem"))); !errors.Is(err, ErrFormat) || !strings.Contains(err.Error(), "Parse400") {
		t.Errorf("400 as 240: %v", err)
	}
	if _, err := Parse(errReader{}); err == nil {
		t.Error("reader error ignored")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestCheckTotals(t *testing.T) {
	l := lines240(t)
	// 5 titles totalling 150000+89990+25000+42050+10000 = 317040.
	good := patch(l[13], 24, "000005"+"00000000000317040")
	l[13] = good
	if _, err := (Options{CheckTotals: true}).Parse(join(l)); err != nil {
		t.Fatalf("consistent totals rejected: %v", err)
	}
	l[13] = patch(good, 24, "000004")
	if _, err := (Options{CheckTotals: true}).Parse(join(l)); !errors.Is(err, ErrCount) {
		t.Errorf("quantity mismatch: %v", err)
	}
	l[13] = patch(good, 30, "00000000000317041")
	if _, err := (Options{CheckTotals: true}).Parse(join(l)); !errors.Is(err, ErrCount) {
		t.Errorf("value mismatch: %v", err)
	}
	if _, err := Parse(join(l)); err != nil {
		t.Errorf("totals checked by default: %v", err)
	}
}

func TestTolerance(t *testing.T) {
	data := read(t, "remessa_341.rem")
	for name, in := range map[string][]byte{
		"ctrl-z":    append(append([]byte(nil), data...), 0x1a),
		"no final":  bytes.TrimSuffix(data, []byte("\r\n")),
		"lf":        bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")),
		"mixed eol": bytes.Replace(data, []byte("\r\n"), []byte("\n"), 3),
	} {
		if _, err := Parse(bytes.NewReader(in)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestRawSegmentsRoundTrip(t *testing.T) {
	l := lines240(t)
	// Turn the R segment into an unknown segment code "Y" and add a type 4
	// record: both must be preserved verbatim.
	l[6] = patch(l[6], 14, "Y")
	rec4 := patch(strings.Repeat(" ", 240), 1, "341000142")
	l = append(l[:13:13], append([]string{rec4}, l[13:]...)...)
	l[14] = patch(l[14], 18, "000014")
	l[15] = patch(l[15], 24, "000016")
	in := strings.Join(l, "\r\n") + "\r\n"
	f, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	segs := f.Batches[0].Segments
	if raw, ok := segs[4].(*cnab240.RawSegment); !ok || raw.Code() != 'Y' {
		t.Fatalf("segment 5 = %T", segs[4])
	}
	if raw, ok := segs[11].(*cnab240.RawSegment); !ok || raw.Code() != 0 {
		t.Fatalf("segment 12 = %T", segs[11])
	}
	var buf bytes.Buffer
	if err := Write(&buf, f); err != nil {
		t.Fatal(err)
	}
	if buf.String() != in {
		t.Fatal("raw segments not preserved")
	}
}

func TestWriteRegenerates(t *testing.T) {
	f, err := Parse(bytes.NewReader(read(t, "remessa_341.rem")))
	if err != nil {
		t.Fatal(err)
	}
	// Scramble control fields and duplicate the batch: Write must fix all.
	b2 := *f.Batches[0]
	b2.Header.Lote = 77
	f.Batches = append(f.Batches, &b2)
	f.Trailer.QuantidadeLotes = 0
	f.Batches[0].Trailer.QuantidadeRegistros = 1
	f.Batches[0].Segments[0].(*cnab240.SegmentP).Sequencial = 40
	var buf bytes.Buffer
	if err := Write(&buf, f); err != nil {
		t.Fatal(err)
	}
	g, err := Parse(&buf)
	if err != nil {
		t.Fatalf("regenerated file invalid: %v", err)
	}
	if len(g.Batches) != 2 || g.Trailer.QuantidadeRegistros != 28 || g.Batches[1].Header.Lote != 2 {
		t.Errorf("trailer %+v", g.Trailer)
	}
	if f.Batches[0].Segments[0].(*cnab240.SegmentP).Sequencial != 40 {
		t.Error("Write modified its input")
	}
}

func TestWriteErrors(t *testing.T) {
	f, _ := Parse(bytes.NewReader(read(t, "remessa_341.rem")))
	bad := func(mut func(*cnab240.File)) error {
		g := *f
		g.Batches = []*cnab240.Batch{{Header: f.Batches[0].Header, Segments: append([]cnab240.Segment(nil), f.Batches[0].Segments...), Trailer: f.Batches[0].Trailer}}
		mut(&g)
		return Write(&bytes.Buffer{}, &g)
	}
	cases := map[string]func(*cnab240.File){
		"header":        func(g *cnab240.File) { g.Header.Agencia = "x" },
		"batch header":  func(g *cnab240.File) { g.Batches[0].Header.Agencia = "x" },
		"segment":       func(g *cnab240.File) { g.Batches[0].Segments[0] = &cnab240.SegmentP{Agencia: "x"} },
		"raw":           func(g *cnab240.File) { g.Batches[0].Segments[0] = &cnab240.RawSegment{Line: "x"} },
		"batch trailer": func(g *cnab240.File) { g.Batches[0].Trailer.NumeroAvisoLancamento = "x" },
		"file trailer":  func(g *cnab240.File) { g.Trailer.CNAB2 = strings.Repeat("x", 300) },
		"too many": func(g *cnab240.File) {
			g.Batches = make([]*cnab240.Batch, 9999)
		},
		"too many segments": func(g *cnab240.File) {
			g.Batches[0].Segments = make([]cnab240.Segment, 100000)
		},
		"too many records": func(g *cnab240.File) {
			segs := make([]cnab240.Segment, 99999)
			for i := range segs {
				segs[i] = &cnab240.RawSegment{Line: strings.Repeat("4", 240)}
			}
			g.Batches = []*cnab240.Batch{{Segments: segs}, {Segments: segs}, {Segments: segs}, {Segments: segs}, {Segments: segs}, {Segments: segs}, {Segments: segs}, {Segments: segs}, {Segments: segs}, {Segments: segs}, {Segments: segs}}
		},
	}
	for name, mut := range cases {
		if err := bad(mut); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestParse400(t *testing.T) {
	data := read(t, "remessa_237.rem")
	f, err := Parse400(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if f.Header.Banco != "237" || f.Header.LiteralServico != "COBRANCA" || len(f.Details) != 2 || !f.Header.DataGravacao.Equal(d(2026, 11, 20)) {
		t.Fatalf("%+v", f.Header)
	}
	var det bradesco.DetalheRemessa
	if err := f.Details[1].Decode(&det); err != nil {
		t.Fatal(err)
	}
	if det.NossoNumero != "00000000102" || det.Valor != 24000 || det.Sequencial != 3 {
		t.Errorf("detail %+v", det)
	}
	var buf bytes.Buffer
	if err := Write400(&buf, f); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Fatal("Write400(Parse400) differs")
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\r\n"), "\r\n")
	bad := map[string][]string{
		"seq":           {lines[0], lines[2], lines[1], lines[3]},
		"short":         {lines[0], lines[1][:399]},
		"one line":      {lines[0]},
		"no header":     {lines[1], patch(lines[1], 395, "000002"), patch(lines[3], 395, "000003")},
		"no trailer":    {lines[0], patch(lines[1], 395, "000002")},
		"middle 9":      {lines[0], patch(lines[3], 395, "000002"), patch(lines[3], 395, "000003")},
		"header field":  {patch(lines[0], 95, "999999"), lines[1], lines[2], lines[3]},
		"trailer field": {lines[0], lines[1], lines[2], patch(lines[3], 1, "9")[:394] + "00000X"},
		"240":           lines240(t),
	}
	for name, ls := range bad {
		if _, err := Parse400(join(ls)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := Parse400(strings.NewReader("")); !errors.Is(err, ErrEmpty) {
		t.Errorf("empty: %v", err)
	}
	if _, err := Parse400(errReader{}); err == nil {
		t.Error("reader error ignored")
	}
	f.Details[0].Dados = "x\n"
	if err := Write400(&bytes.Buffer{}, f); err == nil {
		t.Error("Write400 accepted a newline")
	}
	g := *f
	g.Header.Banco = "x"
	if err := Write400(&bytes.Buffer{}, &g); err == nil {
		t.Error("Write400 accepted bad header")
	}
	g = *f
	g.Details = nil
	g.Trailer.Reservado = strings.Repeat("x", 394)
	if err := Write400(&bytes.Buffer{}, &g); err == nil {
		t.Error("Write400 accepted bad trailer")
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		in   []byte
		want Format
	}{
		{read(t, "remessa_341.rem"), Format240},
		{read(t, "remessa_237.rem"), Format400},
		{[]byte(strings.Repeat("x", 240)), Format240},
		{[]byte("abc\n"), FormatUnknown},
	}
	for _, tt := range tests {
		if got := Detect(tt.in); got != tt.want {
			t.Errorf("Detect = %v, want %v", got, tt.want)
		}
	}
	for f, s := range map[Format]string{Format240: "CNAB 240", Format400: "CNAB 400", FormatUnknown: "unknown"} {
		if f.String() != s {
			t.Errorf("%d.String() = %q", f, f.String())
		}
	}
	le := &LineError{Line: 2, Err: ErrCount}
	if le.Error() != "cnab: line 2: cnab: trailer count mismatch" {
		t.Errorf("LineError = %q", le.Error())
	}
}

// bigRetorno builds a valid retorno with n T/U pairs (2n+4 lines).
func bigRetorno(tb testing.TB, n int) []byte {
	f, err := Parse(bytes.NewReader(read(tb, "retorno_341.ret")))
	if err != nil {
		tb.Fatal(err)
	}
	b := f.Batches[0]
	tpl := b.Segments[6:8]
	b.Segments = make([]cnab240.Segment, 0, 2*n)
	for i := 0; i < n; i++ {
		tt := *tpl[0].(*cnab240.SegmentT)
		tt.NossoNumero = fmt.Sprintf("%08d", i+1)
		b.Segments = append(b.Segments, &tt, tpl[1])
	}
	var buf bytes.Buffer
	if err := Write(&buf, f); err != nil {
		tb.Fatal(err)
	}
	return buf.Bytes()
}

func BenchmarkParse10k(b *testing.B) {
	data := bigRetorno(b, 4998) // 10,000 lines
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Parse(bytes.NewReader(data)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWrite10k(b *testing.B) {
	f, err := Parse(bytes.NewReader(bigRetorno(b, 4998)))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := Write(discard{}, f); err != nil {
			b.Fatal(err)
		}
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func FuzzParse(f *testing.F) {
	f.Add(read(f, "remessa_341.rem"))
	f.Add(read(f, "retorno_341.ret"))
	f.Add(read(f, "remessa_237.rem"))
	f.Add([]byte("\r\n\x1a"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if f4, err := Parse400(bytes.NewReader(data)); err == nil {
			var buf bytes.Buffer
			if err := Write400(&buf, f4); err == nil {
				if _, err := Parse400(&buf); err != nil {
					t.Fatalf("Parse400(Write400(x)) failed: %v", err)
				}
			}
		}
		file, err := Parse(bytes.NewReader(data))
		if err != nil {
			return
		}
		var buf bytes.Buffer
		if err := Write(&buf, file); err != nil {
			return // e.g. lowercase alfa text the encoder rejects
		}
		first := buf.String()
		g, err := Parse(&buf)
		if err != nil {
			t.Fatalf("Parse(Write(Parse(x))) failed: %v", err)
		}
		buf.Reset()
		if err := Write(&buf, g); err != nil || buf.String() != first {
			t.Fatalf("Write not idempotent (%v)", err)
		}
	})
}
