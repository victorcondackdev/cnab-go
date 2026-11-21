package reconcile

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/victorcondackdev/cnab-go/cnab240"
)

func d(dd int) time.Time { return time.Date(2026, 12, dd, 0, 0, 0, 0, time.UTC) }

func liq(nn string, pago int64, on time.Time) Entry {
	return Entry{NossoNumero: nn, Movimento: cnab240.RetornoLiquidacao, ValorPago: pago, ValorLiquido: pago, DataOcorrencia: on}
}

func TestReconcile(t *testing.T) {
	expected := []Expected{
		{NossoNumero: "0001", Valor: 1000, Vencimento: d(10)},
		{NossoNumero: "2", Valor: 1000, Vencimento: d(10)},
		{NossoNumero: "3", Valor: 1000, Vencimento: d(10)},
		{NossoNumero: "4", Valor: 1000, Vencimento: d(10)},
		{NossoNumero: "5", Valor: 1000, Vencimento: d(10)},
		{NossoNumero: "6", Valor: 1000, Vencimento: d(10)},
		{NossoNumero: "7", Valor: 1000, Vencimento: d(10)},
		{NossoNumero: "8", Valor: 1000, Vencimento: d(10)},
	}
	adj := liq("2", 900, d(9))
	adj.Desconto = 100
	late := liq("3", 1050, d(12))
	late.JurosMulta = 50
	div := liq("4", 1200, d(10))
	div.JurosMulta = 50
	lateNoInterest := liq("6", 1000, d(11))
	nominal := liq("7", 1000, d(10))
	nominal.Valor = 999
	creditOnly := liq("8", 1000, time.Time{})
	creditOnly.DataCredito = d(10)
	entries := []Entry{
		{NossoNumero: "00000001  ", Movimento: cnab240.RetornoEntradaConfirmada},
		liq("1", 1000, d(10)),
		adj, late, div,
		liq("1", 1000, d(11)),
		liq("99", 500, d(10)),
		{NossoNumero: "5", Movimento: cnab240.RetornoEntradaRejeitada, Motivos: []string{"08"}},
		{NossoNumero: "77", Movimento: cnab240.RetornoInstrucaoRejeitada},
		lateNoInterest, nominal, creditOnly,
		{NossoNumero: "2", Movimento: cnab240.RetornoDebitoTarifas, Tarifa: 350},
	}
	rep, err := Reconcile(expected, entries)
	if err != nil {
		t.Fatal(err)
	}
	want := []Status{StatusInformational, StatusPaid, StatusPaidAdjusted, StatusPaidLate, StatusDivergent, StatusDuplicate,
		StatusUnknown, StatusRejected, StatusRejected, StatusPaidLate, StatusPaid, StatusPaid, StatusInformational, StatusOpen}
	if len(rep.Results) != len(want) {
		t.Fatalf("got %d results: %+v", len(rep.Results), rep.Results)
	}
	for i, w := range want {
		if r := rep.Results[i]; r.Status != w {
			t.Errorf("result %d (%s) = %s, want %s: %s", i, r.NossoNumero, r.Status, w, r.Explanation)
		}
	}
	checks := map[int]string{
		2:  "explained by desconto R$ 1,00",
		3:  "2 day(s) after due date",
		4:  "explain R$ 0,50 (juros/multa R$ 0,50)",
		5:  "already settled on 2026-12-10",
		7:  "motivos 08",
		8:  "nosso número not expected",
		9:  "no interest charged",
		10: "bank reports nominal value R$ 9,99",
		13: "no settlement in this retorno; due 2026-12-10",
	}
	for i, s := range checks {
		if !strings.Contains(rep.Results[i].Explanation, s) {
			t.Errorf("result %d explanation %q lacks %q", i, rep.Results[i].Explanation, s)
		}
	}
	if rep.Results[4].Difference != 200 || rep.Results[2].Difference != -100 {
		t.Error("differences")
	}
	if rep.ExpectedTotal != 8000 || rep.PaidTotal != 1000+900+1050+1200+1000+1000+1000 || rep.TariffTotal != 350 {
		t.Errorf("totals %+v", rep)
	}
	if !rep.NeedsAttention() || rep.Count(StatusPaid) != 3 || len(rep.Filter(StatusRejected)) != 2 {
		t.Error("helpers")
	}
	s := rep.Summary()
	for _, want := range []string{"13 retorno entries, 8 expected boletos", "paid exact                3", "[divergent] 4:", "Expected R$ 80,00"} {
		if !strings.Contains(s, want) {
			t.Errorf("summary lacks %q:\n%s", want, s)
		}
	}
	// Grace days turn the late payments into on-time ones.
	rep, _ = Options{GraceDays: 2}.Reconcile(expected, entries)
	if rep.Results[3].Status != StatusPaidAdjusted || rep.Results[9].Status != StatusPaid {
		t.Errorf("grace days: %s %s", rep.Results[3].Status, rep.Results[9].Status)
	}
}

func TestReconcileClean(t *testing.T) {
	rep, err := Reconcile([]Expected{{NossoNumero: "1", Valor: 10}}, []Entry{liq("1", 10, time.Time{})})
	if err != nil || rep.NeedsAttention() || strings.Contains(rep.Summary(), "Attention") {
		t.Fatalf("%v %s", err, rep.Summary())
	}
	if !strings.Contains(rep.Results[0].Explanation, "(no date)") {
		t.Error(rep.Results[0].Explanation)
	}
}

func TestDuplicateExpected(t *testing.T) {
	_, err := Reconcile([]Expected{{NossoNumero: "01"}, {NossoNumero: "1 "}}, nil)
	if !errors.Is(err, ErrDuplicateExpected) {
		t.Fatal(err)
	}
}

func TestKey(t *testing.T) {
	for in, want := range map[string]string{"": "", "   ": "", "0": "0", "0000": "0", "000123  ": "123", "12 34": "1234", "A0": "A0"} {
		if got := Key(in); got != want {
			t.Errorf("Key(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStatusText(t *testing.T) {
	b, err := json.Marshal(map[string]Status{"s": StatusPaidLate})
	if err != nil || string(b) != `{"s":"paid_late"}` {
		t.Errorf("%s %v", b, err)
	}
	if Status(0).String() != "status(0)" {
		t.Error(Status(0).String())
	}
}

func TestEntryFrom(t *testing.T) {
	o := cnab240.Ocorrencia{T: &cnab240.SegmentT{NossoNumero: "1", Movimento: 6, Valor: 10, Tarifa: 2, Motivos: "01"}}
	e := EntryFrom(o)
	if e.NossoNumero != "1" || e.Valor != 10 || e.Tarifa != 2 || e.ValorPago != 0 || len(e.Motivos) != 1 {
		t.Errorf("%+v", e)
	}
	o.U = &cnab240.SegmentU{ValorPago: 12, ValorLiquido: 10, JurosMulta: 2, Desconto: 1, Abatimento: 1, DataOcorrencia: d(1), DataCredito: d(2)}
	e = EntryFrom(o)
	if e.ValorPago != 12 || e.ValorLiquido != 10 || e.JurosMulta != 2 || !e.DataCredito.Equal(d(2)) {
		t.Errorf("%+v", e)
	}
	f := &cnab240.File{Batches: []*cnab240.Batch{{Segments: []cnab240.Segment{o.T, o.U}}}}
	if es := EntriesFromFile(f); len(es) != 1 || es[0].ValorPago != 12 {
		t.Errorf("%+v", es)
	}
}
