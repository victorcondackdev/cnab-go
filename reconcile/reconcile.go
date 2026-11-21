// Package reconcile matches the occurrences of a CNAB 240 retorno file
// against the boletos a company expects to receive, and explains every
// difference.
//
// Each retorno occurrence becomes an [Entry]. [Reconcile] classifies each
// one as paid exactly, paid with an explained adjustment (interest, fine,
// discount, abatement), paid late, paid with an unexplained divergence, a
// duplicate settlement, a settlement for an unknown nosso número, a
// rejection, or a purely informational occurrence. Expected boletos with no
// settlement are reported as open. Amounts are int64 cents throughout, so
// totals are exact.
//
// Nosso número values are matched after removing spaces and leading zeros,
// because banks pad the 20-position field differently in remessa and
// retorno files.
package reconcile

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/victorcondackdev/cnab-go/cnab240"
	"github.com/victorcondackdev/cnab-go/internal/money"
)

// ErrDuplicateExpected reports two expected boletos with the same nosso
// número.
var ErrDuplicateExpected = errors.New("reconcile: duplicate expected nosso número")

// Expected is a boleto the company issued and expects to be paid.
type Expected struct {
	NossoNumero string
	Valor       int64 // cents
	Vencimento  time.Time
	Referencia  string // free text: invoice, order, customer id
}

// Entry is one retorno occurrence (segment T plus its U).
type Entry struct {
	NossoNumero     string
	NumeroDocumento string
	Movimento       cnab240.MovimentoRetorno
	Motivos         []string
	Vencimento      time.Time
	Valor           int64 // nominal value reported by the bank
	ValorPago       int64
	ValorLiquido    int64
	JurosMulta      int64
	Desconto        int64
	Abatimento      int64
	Tarifa          int64
	DataOcorrencia  time.Time
	DataCredito     time.Time
}

// EntryFrom converts a retorno occurrence. U may be nil.
func EntryFrom(o cnab240.Ocorrencia) Entry {
	t := o.T
	e := Entry{
		NossoNumero:     t.NossoNumero,
		NumeroDocumento: t.NumeroDocumento,
		Movimento:       t.Movimento,
		Motivos:         t.MotivoCodes(),
		Vencimento:      t.Vencimento,
		Valor:           t.Valor,
		Tarifa:          t.Tarifa,
	}
	if u := o.U; u != nil {
		e.ValorPago, e.ValorLiquido = u.ValorPago, u.ValorLiquido
		e.JurosMulta, e.Desconto, e.Abatimento = u.JurosMulta, u.Desconto, u.Abatimento
		e.DataOcorrencia, e.DataCredito = u.DataOcorrencia, u.DataCredito
	}
	return e
}

// EntriesFromFile converts every occurrence of a retorno file, in order.
func EntriesFromFile(f *cnab240.File) []Entry {
	occ := f.Ocorrencias()
	out := make([]Entry, len(occ))
	for i, o := range occ {
		out[i] = EntryFrom(o)
	}
	return out
}

// Status classifies a reconciliation result.
type Status int

// Result statuses, in report order.
const (
	StatusPaid          Status = iota + 1 // paid the expected amount on time
	StatusPaidAdjusted                    // paid on time; difference fully explained by juros/desconto/abatimento
	StatusPaidLate                        // paid after the due date (plus grace days)
	StatusDivergent                       // paid, but the difference is not explained by the reported adjustments
	StatusDuplicate                       // second settlement of an already settled boleto
	StatusUnknown                         // settlement for a nosso número not in the expected list
	StatusRejected                        // the bank rejected an entry or instruction
	StatusOpen                            // expected boleto without settlement in this retorno
	StatusInformational                   // other occurrences (entry confirmed, tariffs, baixa, ...)
)

var statusNames = map[Status]string{
	StatusPaid: "paid", StatusPaidAdjusted: "paid_adjusted", StatusPaidLate: "paid_late",
	StatusDivergent: "divergent", StatusDuplicate: "duplicate", StatusUnknown: "unknown",
	StatusRejected: "rejected", StatusOpen: "open", StatusInformational: "informational",
}

// String returns a stable snake_case name, suitable for JSON and CSV.
func (s Status) String() string {
	if n, ok := statusNames[s]; ok {
		return n
	}
	return fmt.Sprintf("status(%d)", int(s))
}

// MarshalText implements encoding.TextMarshaler.
func (s Status) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// Result is the outcome for one retorno entry or one open expected boleto.
// Expected or Entry is nil when there is none. Difference is the amount
// paid minus the amount expected (settlements only).
type Result struct {
	Status      Status
	NossoNumero string
	Expected    *Expected
	Entry       *Entry
	Difference  int64
	Explanation string
}

// Options tunes the classification. The zero value is valid.
type Options struct {
	// GraceDays tolerates payments up to this many days after the due date
	// before classifying them as late (for example, due dates falling on
	// weekends or holidays are payable on the next business day).
	GraceDays int
}

// Report is the outcome of a reconciliation.
type Report struct {
	Results []Result

	ExpectedTotal int64 // sum of expected values
	PaidTotal     int64 // sum of ValorPago over accepted settlements (not duplicates/unknown)
	CreditedTotal int64 // sum of ValorLiquido over all settlements
	TariffTotal   int64 // sum of tariffs reported on any entry
	entries       int
	expected      int
}

// Reconcile matches entries against expected with default [Options].
func Reconcile(expected []Expected, entries []Entry) (*Report, error) {
	return Options{}.Reconcile(expected, entries)
}

// Key normalizes a nosso número for matching: spaces and leading zeros are
// removed ("00000123 " and "123" match).
func Key(nn string) string {
	k := strings.ReplaceAll(nn, " ", "")
	k = strings.TrimLeft(k, "0")
	if k == "" && nn != "" && strings.Contains(nn, "0") {
		return "0"
	}
	return k
}

// Reconcile matches entries against expected.
func (o Options) Reconcile(expected []Expected, entries []Entry) (*Report, error) {
	byKey := make(map[string]*Expected, len(expected))
	rep := &Report{entries: len(entries), expected: len(expected)}
	for i := range expected {
		e := &expected[i]
		k := Key(e.NossoNumero)
		if _, dup := byKey[k]; dup {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateExpected, e.NossoNumero)
		}
		byKey[k] = e
		rep.ExpectedTotal += e.Valor
	}
	settled := make(map[string]*Entry)
	for i := range entries {
		en := &entries[i]
		k := Key(en.NossoNumero)
		exp := byKey[k]
		rep.TariffTotal += en.Tarifa
		r := Result{NossoNumero: strings.TrimSpace(en.NossoNumero), Expected: exp, Entry: en}
		switch {
		case en.Movimento.IsLiquidacao():
			rep.CreditedTotal += en.ValorLiquido
			if prev, ok := settled[k]; ok {
				r.Status = StatusDuplicate
				r.Difference = en.ValorPago
				r.Explanation = fmt.Sprintf("already settled on %s for %s; this settlement of %s is a duplicate",
					day(prev.DataOcorrencia), money.FormatBRL(prev.ValorPago), money.FormatBRL(en.ValorPago))
				break
			}
			settled[k] = en
			if exp == nil {
				r.Status = StatusUnknown
				r.Difference = en.ValorPago
				r.Explanation = fmt.Sprintf("settlement of %s for a nosso número that is not expected", money.FormatBRL(en.ValorPago))
				break
			}
			rep.PaidTotal += en.ValorPago
			o.classifyPayment(&r, exp, en)
		case en.Movimento.IsRejeicao():
			r.Status = StatusRejected
			r.Explanation = en.Movimento.String()
			if len(en.Motivos) > 0 {
				r.Explanation += "; motivos " + strings.Join(en.Motivos, ", ") + " (see the bank's manual)"
			}
			if exp == nil {
				r.Explanation += "; nosso número not expected"
			}
		default:
			r.Status = StatusInformational
			r.Explanation = en.Movimento.String()
		}
		rep.Results = append(rep.Results, r)
	}
	for i := range expected {
		e := &expected[i]
		if _, ok := settled[Key(e.NossoNumero)]; ok {
			continue
		}
		rep.Results = append(rep.Results, Result{
			Status: StatusOpen, NossoNumero: e.NossoNumero, Expected: e,
			Explanation: fmt.Sprintf("no settlement in this retorno; due %s", day(e.Vencimento)),
		})
	}
	return rep, nil
}

func day(t time.Time) string {
	if t.IsZero() {
		return "(no date)"
	}
	return t.Format(time.DateOnly)
}

func (o Options) classifyPayment(r *Result, exp *Expected, en *Entry) {
	r.Difference = en.ValorPago - exp.Valor
	explained := en.JurosMulta - en.Desconto - en.Abatimento
	paidOn := en.DataOcorrencia
	if paidOn.IsZero() {
		paidOn = en.DataCredito
	}
	late := !exp.Vencimento.IsZero() && !paidOn.IsZero() &&
		paidOn.After(exp.Vencimento.AddDate(0, 0, o.GraceDays))
	var parts []string
	if en.JurosMulta != 0 {
		parts = append(parts, "juros/multa "+money.FormatBRL(en.JurosMulta))
	}
	if en.Desconto != 0 {
		parts = append(parts, "desconto "+money.FormatBRL(en.Desconto))
	}
	if en.Abatimento != 0 {
		parts = append(parts, "abatimento "+money.FormatBRL(en.Abatimento))
	}
	adj := strings.Join(parts, ", ")
	diff := fmt.Sprintf("paid %s, expected %s (difference %s)",
		money.FormatBRL(en.ValorPago), money.FormatBRL(exp.Valor), money.FormatBRL(r.Difference))
	switch {
	case r.Difference != explained:
		r.Status = StatusDivergent
		r.Explanation = diff + "; adjustments reported by the bank explain " + money.FormatBRL(explained)
		if adj != "" {
			r.Explanation += " (" + adj + ")"
		}
	case late:
		r.Status = StatusPaidLate
		days := int(paidOn.Sub(exp.Vencimento).Hours() / 24)
		r.Explanation = fmt.Sprintf("paid on %s, %d day(s) after due date %s", day(paidOn), days, day(exp.Vencimento))
		if r.Difference == 0 {
			r.Explanation += "; no interest charged"
		} else {
			r.Explanation += "; " + diff + " explained by " + adj
		}
	case r.Difference != 0:
		r.Status = StatusPaidAdjusted
		r.Explanation = diff + " explained by " + adj
	default:
		r.Status = StatusPaid
		r.Explanation = "paid " + money.FormatBRL(en.ValorPago) + " on " + day(paidOn)
	}
	if en.Valor != 0 && en.Valor != exp.Valor {
		r.Explanation += fmt.Sprintf("; note: bank reports nominal value %s", money.FormatBRL(en.Valor))
	}
}

// Count returns the number of results with status s.
func (r *Report) Count(s Status) int {
	n := 0
	for _, res := range r.Results {
		if res.Status == s {
			n++
		}
	}
	return n
}

// Filter returns the results with status s.
func (r *Report) Filter(s Status) []Result {
	var out []Result
	for _, res := range r.Results {
		if res.Status == s {
			out = append(out, res)
		}
	}
	return out
}

// NeedsAttention reports whether any result is divergent, duplicate,
// unknown or rejected.
func (r *Report) NeedsAttention() bool {
	for _, res := range r.Results {
		switch res.Status {
		case StatusDivergent, StatusDuplicate, StatusUnknown, StatusRejected:
			return true
		}
	}
	return false
}

// Summary renders a human-readable overview: counts per status, totals and
// the explanation of every result that needs attention.
func (r *Report) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Reconciliation: %d retorno entries, %d expected boletos\n", r.entries, r.expected)
	labels := []struct {
		s Status
		l string
	}{
		{StatusPaid, "paid exact"}, {StatusPaidAdjusted, "paid with adjustment"}, {StatusPaidLate, "paid late"},
		{StatusDivergent, "divergent"}, {StatusDuplicate, "duplicate"}, {StatusUnknown, "unknown nosso número"},
		{StatusRejected, "rejected"}, {StatusOpen, "open"}, {StatusInformational, "informational"},
	}
	for _, l := range labels {
		fmt.Fprintf(&b, "  %-22s %4d\n", l.l, r.Count(l.s))
	}
	fmt.Fprintf(&b, "Expected %s, paid %s, credited %s, tariffs %s\n",
		money.FormatBRL(r.ExpectedTotal), money.FormatBRL(r.PaidTotal),
		money.FormatBRL(r.CreditedTotal), money.FormatBRL(r.TariffTotal))
	first := true
	for _, res := range r.Results {
		switch res.Status {
		case StatusDivergent, StatusDuplicate, StatusUnknown, StatusRejected, StatusPaidLate:
			if first {
				b.WriteString("Attention:\n")
				first = false
			}
			fmt.Fprintf(&b, "  [%s] %s: %s\n", res.Status, res.NossoNumero, res.Explanation)
		}
	}
	return b.String()
}
