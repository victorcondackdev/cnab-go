// Command cnab inspects, validates and reconciles FEBRABAN CNAB files.
//
// Usage:
//
//	cnab inspect [--json] FILE
//	cnab validate [--totals] FILE...
//	cnab reconcile --expected expected.csv [--grace-days N] [--strict] [--json] RETORNO
//
// The expected CSV needs a header row with the columns nosso_numero, valor
// (decimal, "1234.56" or "1.234,56") and vencimento (YYYY-MM-DD or
// DD/MM/YYYY); an optional referencia column is carried into the report.
// Comma and semicolon delimiters are detected automatically.
//
// Exit status is 0 on success, 1 when a file is invalid (or, with
// --strict, when the reconciliation needs attention) and 2 on usage errors.
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/victorcondackdev/cnab-go/cnab240"
	"github.com/victorcondackdev/cnab-go/file"
	"github.com/victorcondackdev/cnab-go/internal/money"
	"github.com/victorcondackdev/cnab-go/reconcile"
)

const usage = `cnab inspects, validates and reconciles FEBRABAN CNAB files.

Usage:
  cnab inspect [--json] FILE
  cnab validate [--totals] FILE...
  cnab reconcile --expected expected.csv [--grace-days N] [--strict] [--json] RETORNO
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "inspect":
		return cmdInspect(args[1:], stdout, stderr)
	case "validate":
		return cmdValidate(args[1:], stdout, stderr)
	case "reconcile":
		return cmdReconcile(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	}
	fmt.Fprintf(stderr, "cnab: unknown command %q\n\n%s", args[0], usage)
	return 2
}

func newFlags(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("cnab "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func date(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.DateOnly)
}

// --- inspect ---

type batchSummary struct {
	Lote        int            `json:"lote"`
	Operacao    string         `json:"operacao"`
	Records     int            `json:"records"`
	Segments    map[string]int `json:"segments"`
	Titles      int            `json:"titles"`
	Nominal     int64          `json:"nominal_cents"`
	Paid        int64          `json:"paid_cents"`
	Credited    int64          `json:"credited_cents"`
	Tariffs     int64          `json:"tariffs_cents"`
	Movimentos  map[string]int `json:"movimentos"`
	movimentoIx []string
}

type fileSummary struct {
	Format  string         `json:"format"`
	Banco   string         `json:"banco"`
	Empresa string         `json:"empresa"`
	Tipo    string         `json:"tipo"`
	Gerado  string         `json:"gerado_em,omitempty"`
	NSA     int            `json:"nsa,omitempty"`
	Lines   int            `json:"lines"`
	Batches []batchSummary `json:"batches,omitempty"`
	Details int            `json:"details,omitempty"`
}

func summarize240(f *cnab240.File) fileSummary {
	s := fileSummary{Format: "CNAB 240", Banco: f.Header.Banco, Empresa: f.Header.NomeEmpresa,
		Gerado: date(f.Header.DataGeracao), NSA: f.Header.NSA, Lines: f.Trailer.QuantidadeRegistros}
	switch f.Header.CodigoRemessaRetorno {
	case cnab240.CodigoRemessa:
		s.Tipo = "remessa"
	case cnab240.CodigoRetorno:
		s.Tipo = "retorno"
	default:
		s.Tipo = fmt.Sprintf("código %d", f.Header.CodigoRemessaRetorno)
	}
	for _, b := range f.Batches {
		bs := batchSummary{Lote: b.Header.Lote, Operacao: b.Header.Operacao, Records: len(b.Segments) + 2,
			Segments: map[string]int{}, Movimentos: map[string]int{}}
		count := func(code string) {
			if _, ok := bs.Movimentos[code]; !ok {
				bs.movimentoIx = append(bs.movimentoIx, code)
			}
			bs.Movimentos[code]++
		}
		for _, seg := range b.Segments {
			c := "raw"
			if seg.Code() != 0 {
				c = string(seg.Code())
			}
			bs.Segments[c]++
			switch seg := seg.(type) {
			case *cnab240.SegmentP:
				bs.Titles++
				bs.Nominal += seg.Valor
				count(fmt.Sprintf("%02d %s", int(seg.Movimento), seg.Movimento))
			case *cnab240.SegmentT:
				bs.Titles++
				bs.Nominal += seg.Valor
				bs.Tariffs += seg.Tarifa
				count(fmt.Sprintf("%02d %s", int(seg.Movimento), seg.Movimento))
			case *cnab240.SegmentU:
				if seg.Movimento.IsLiquidacao() {
					bs.Paid += seg.ValorPago
					bs.Credited += seg.ValorLiquido
				}
			}
		}
		sort.Strings(bs.movimentoIx)
		s.Batches = append(s.Batches, bs)
	}
	return s
}

func cmdInspect(args []string, stdout, stderr io.Writer) int {
	fs := newFlags("inspect", stderr)
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: cnab inspect [--json] FILE")
		return 2
	}
	data, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "cnab:", err)
		return 1
	}
	var s fileSummary
	switch file.Detect(data) {
	case file.Format400:
		f, err := file.Parse400(bytes.NewReader(data))
		if err != nil {
			fmt.Fprintln(stderr, "cnab:", err)
			return 1
		}
		s = fileSummary{Format: "CNAB 400", Banco: f.Header.Banco, Empresa: f.Header.NomeEmpresa,
			Tipo: strings.ToLower(f.Header.LiteralOperacao), Gerado: date(f.Header.DataGravacao),
			Lines: len(f.Details) + 2, Details: len(f.Details)}
	default:
		f, err := file.Parse(bytes.NewReader(data))
		if err != nil {
			fmt.Fprintln(stderr, "cnab:", err)
			return 1
		}
		s = summarize240(f)
	}
	if *asJSON {
		return writeJSON(stdout, stderr, s)
	}
	fmt.Fprintf(stdout, "%s %s, banco %s, empresa %s\n", s.Format, s.Tipo, s.Banco, s.Empresa)
	fmt.Fprintf(stdout, "gerado em %s, ", s.Gerado)
	if s.NSA > 0 {
		fmt.Fprintf(stdout, "NSA %d, ", s.NSA)
	}
	fmt.Fprintf(stdout, "%d lines\n", s.Lines)
	if s.Format == "CNAB 400" {
		fmt.Fprintf(stdout, "%d detail records (decode with a bank-specific layout)\n", s.Details)
		return 0
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "lote\top\trecords\tsegments\ttitles\tnominal\tpaid\tcredited\ttariffs\t")
	for _, b := range s.Batches {
		var segs []string
		for c, n := range b.Segments {
			segs = append(segs, fmt.Sprintf("%s:%d", c, n))
		}
		sort.Strings(segs)
		fmt.Fprintf(tw, "%04d\t%s\t%d\t%s\t%d\t%s\t%s\t%s\t%s\t\n", b.Lote, b.Operacao, b.Records, strings.Join(segs, " "),
			b.Titles, money.Format(b.Nominal), money.Format(b.Paid), money.Format(b.Credited), money.Format(b.Tariffs))
	}
	tw.Flush()
	for _, b := range s.Batches {
		fmt.Fprintf(stdout, "lote %04d movimentos:\n", b.Lote)
		for _, m := range b.movimentoIx {
			fmt.Fprintf(stdout, "  %4d  %s\n", b.Movimentos[m], m)
		}
	}
	return 0
}

func writeJSON(stdout, stderr io.Writer, v any) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(stderr, "cnab:", err)
		return 1
	}
	return 0
}

// --- validate ---

func cmdValidate(args []string, stdout, stderr io.Writer) int {
	fs := newFlags("validate", stderr)
	totals := fs.Bool("totals", false, "also check batch trailer title totals (CNAB 240)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "usage: cnab validate [--totals] FILE...")
		return 2
	}
	status := 0
	for _, name := range fs.Args() {
		data, err := os.ReadFile(name)
		if err == nil {
			switch file.Detect(data) {
			case file.Format400:
				_, err = file.Parse400(bytes.NewReader(data))
			default:
				_, err = file.Options{CheckTotals: *totals}.Parse(bytes.NewReader(data))
			}
		}
		if err != nil {
			fmt.Fprintf(stdout, "FAIL %s: %v\n", name, err)
			status = 1
			continue
		}
		fmt.Fprintf(stdout, "ok   %s (%s)\n", name, file.Detect(data))
	}
	return status
}

// --- reconcile ---

func readExpected(r io.Reader) ([]reconcile.Expected, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // UTF-8 BOM from spreadsheets
	cr := csv.NewReader(bytes.NewReader(data))
	if first, _, _ := bytes.Cut(data, []byte("\n")); bytes.Count(first, []byte(";")) > bytes.Count(first, []byte(",")) {
		cr.Comma = ';'
	}
	cr.TrimLeadingSpace = true
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("expected CSV is empty")
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, c := range []string{"nosso_numero", "valor", "vencimento"} {
		if _, ok := col[c]; !ok {
			return nil, fmt.Errorf("expected CSV: missing column %q", c)
		}
	}
	ref, hasRef := col["referencia"]
	out := make([]reconcile.Expected, 0, len(rows)-1)
	for i, row := range rows[1:] {
		line := i + 2
		v, err := money.Parse(row[col["valor"]])
		if err != nil {
			return nil, fmt.Errorf("expected CSV line %d: valor %q: %w", line, row[col["valor"]], err)
		}
		venc, err := parseDate(row[col["vencimento"]])
		if err != nil {
			return nil, fmt.Errorf("expected CSV line %d: vencimento %q: %w", line, row[col["vencimento"]], err)
		}
		e := reconcile.Expected{NossoNumero: strings.TrimSpace(row[col["nosso_numero"]]), Valor: v, Vencimento: venc}
		if hasRef {
			e.Referencia = row[ref]
		}
		out = append(out, e)
	}
	return out, nil
}

func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.DateOnly, "02/01/2006", "02012006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("want YYYY-MM-DD or DD/MM/YYYY")
}

type jsonResult struct {
	Status         string   `json:"status"`
	NossoNumero    string   `json:"nosso_numero"`
	Referencia     string   `json:"referencia,omitempty"`
	Movimento      string   `json:"movimento,omitempty"`
	ExpectedCents  *int64   `json:"expected_cents,omitempty"`
	PaidCents      *int64   `json:"paid_cents,omitempty"`
	DiffCents      int64    `json:"difference_cents"`
	Vencimento     string   `json:"vencimento,omitempty"`
	DataOcorrencia string   `json:"data_ocorrencia,omitempty"`
	DataCredito    string   `json:"data_credito,omitempty"`
	Motivos        []string `json:"motivos,omitempty"`
	Explanation    string   `json:"explanation"`
}

type jsonReport struct {
	ExpectedTotal  int64          `json:"expected_total_cents"`
	PaidTotal      int64          `json:"paid_total_cents"`
	CreditedTotal  int64          `json:"credited_total_cents"`
	TariffTotal    int64          `json:"tariff_total_cents"`
	Counts         map[string]int `json:"counts"`
	NeedsAttention bool           `json:"needs_attention"`
	Results        []jsonResult   `json:"results"`
}

func toJSON(rep *reconcile.Report) jsonReport {
	out := jsonReport{ExpectedTotal: rep.ExpectedTotal, PaidTotal: rep.PaidTotal, CreditedTotal: rep.CreditedTotal,
		TariffTotal: rep.TariffTotal, Counts: map[string]int{}, NeedsAttention: rep.NeedsAttention(), Results: []jsonResult{}}
	for _, r := range rep.Results {
		jr := jsonResult{Status: r.Status.String(), NossoNumero: r.NossoNumero, DiffCents: r.Difference, Explanation: r.Explanation}
		if e := r.Expected; e != nil {
			v := e.Valor
			jr.ExpectedCents, jr.Referencia, jr.Vencimento = &v, e.Referencia, date(e.Vencimento)
		}
		if e := r.Entry; e != nil {
			jr.Movimento = fmt.Sprintf("%02d %s", int(e.Movimento), e.Movimento)
			jr.DataOcorrencia, jr.DataCredito, jr.Motivos = date(e.DataOcorrencia), date(e.DataCredito), e.Motivos
			if e.Movimento.IsLiquidacao() {
				v := e.ValorPago
				jr.PaidCents = &v
			}
		}
		out.Counts[jr.Status]++
		out.Results = append(out.Results, jr)
	}
	return out
}

func cmdReconcile(args []string, stdout, stderr io.Writer) int {
	fs := newFlags("reconcile", stderr)
	expPath := fs.String("expected", "", "CSV of expected boletos (required)")
	grace := fs.Int("grace-days", 0, "days after the due date still considered on time")
	strict := fs.Bool("strict", false, "exit 1 when any result needs attention")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *expPath == "" || fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: cnab reconcile --expected expected.csv [--grace-days N] [--strict] [--json] RETORNO")
		return 2
	}
	ef, err := os.Open(*expPath)
	if err != nil {
		fmt.Fprintln(stderr, "cnab:", err)
		return 1
	}
	expected, err := readExpected(ef)
	ef.Close()
	if err != nil {
		fmt.Fprintln(stderr, "cnab:", err)
		return 1
	}
	rf, err := os.Open(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "cnab:", err)
		return 1
	}
	f, err := file.Parse(rf)
	rf.Close()
	if err != nil {
		fmt.Fprintln(stderr, "cnab:", err)
		return 1
	}
	rep, err := reconcile.Options{GraceDays: *grace}.Reconcile(expected, reconcile.EntriesFromFile(f))
	if err != nil {
		fmt.Fprintln(stderr, "cnab:", err)
		return 1
	}
	if *asJSON {
		if code := writeJSON(stdout, stderr, toJSON(rep)); code != 0 {
			return code
		}
	} else {
		fmt.Fprint(stdout, rep.Summary())
	}
	if *strict && rep.NeedsAttention() {
		return 1
	}
	return 0
}
