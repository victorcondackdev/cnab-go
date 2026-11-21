package file

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/victorcondackdev/cnab-go/cnab240"
)

var (
	testCompany = Company{Nome: "Acme Utilidades Ltda", Documento: "11.222.333/0001-81",
		Agencia: "01234", Conta: "000000056789", ContaDV: "0"}
	testBank = Bank{Codigo: "341", Nome: "Banco Itaú S.A."}
	pagJoao  = Pagador{Nome: "João da Silva Sauro", Documento: "529.982.247-25", Endereco: "Rua das Laranjeiras, 1500 apto 302",
		Bairro: "Laranjeiras", CEP: "22240-003", Cidade: "Rio de Janeiro", UF: "RJ"}
	pagPadaria = Pagador{Nome: "Padaria Pão Quente Ltda", Documento: "11222333000181", Endereco: "Av. Brasil, 2000",
		Bairro: "Penha", CEP: "21011-000", Cidade: "Rio de Janeiro", UF: "RJ"}
)

// sampleRemessa rebuilds testdata/remessa_341.rem.
func sampleRemessa() *Remessa {
	r := NewRemessa(testCompany, testBank).WithNSA(42).WithGeneratedAt(time.Date(2026, 11, 20, 9, 30, 15, 0, time.UTC))
	em := d(2026, 11, 20)
	r.AddBoleto(Boleto{NossoNumero: "00000001", NumeroDocumento: "NF-1001", Vencimento: d(2026, 12, 10), Valor: 150000, Pagador: pagJoao, JurosPorDia: 50, Emissao: em})
	r.AddBoleto(Boleto{NossoNumero: "00000002", NumeroDocumento: "NF-1002", Vencimento: d(2026, 12, 15), Valor: 89990, Pagador: pagPadaria, JurosPorDia: 30, Multa: 1800, Mensagem: "Não receber após 30 dias do vencimento", Emissao: em, DiasProtesto: 5})
	r.AddBoleto(Boleto{NossoNumero: "00000003", NumeroDocumento: "NF-1003", Vencimento: d(2026, 12, 20), Valor: 25000, Pagador: pagJoao, Desconto: 1000, DescontoAte: d(2026, 12, 10), Emissao: em})
	r.AddBoleto(Boleto{NossoNumero: "00000004", NumeroDocumento: "NF-1004", Vencimento: d(2026, 12, 20), Valor: 42050, Pagador: pagPadaria, Desconto: 2050, Emissao: em})
	r.AddBoleto(Boleto{NossoNumero: "00000005", NumeroDocumento: "NF-1005", Vencimento: d(2026, 12, 30), Valor: 10000, Pagador: pagJoao, Emissao: em})
	return r
}

func TestRemessaBuilderMatchesSample(t *testing.T) {
	var buf bytes.Buffer
	n, err := sampleRemessa().WriteTo(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(buf.Len()) {
		t.Errorf("WriteTo returned %d, wrote %d", n, buf.Len())
	}
	if want := read(t, "remessa_341.rem"); !bytes.Equal(buf.Bytes(), want) {
		t.Fatal("builder output differs from testdata/remessa_341.rem")
	}
}

func TestRemessaBuilderFields(t *testing.T) {
	f, err := sampleRemessa().Build()
	if err != nil {
		t.Fatal(err)
	}
	ts := f.Titulos()
	p := ts[1].P
	if p.CodigoJuros != cnab240.JurosValorDiario || !p.DataJuros.Equal(d(2026, 12, 16)) || p.CodigoProtesto != cnab240.ProtestarDiasCorridos || p.DiasProtesto != 5 {
		t.Errorf("P %+v", p)
	}
	if ts[3].P.CodigoDesconto != cnab240.DescontoValorFixo || !ts[3].P.DataDesconto.Equal(d(2026, 12, 20)) {
		t.Errorf("desconto default date: %+v", ts[3].P)
	}
	if ts[1].Q.TipoInscricao != cnab240.InscricaoCNPJ || ts[0].Q.TipoInscricao != cnab240.InscricaoCPF {
		t.Error("tipo de inscrição not derived from the document")
	}
	if f.Batches[0].Trailer.QuantidadeRegistros != 13 || f.Trailer.QuantidadeRegistros != 15 {
		t.Errorf("counts %d %d", f.Batches[0].Trailer.QuantidadeRegistros, f.Trailer.QuantidadeRegistros)
	}
	// Defaults: emissão = remessa date, carteira 1, aceite, baixa.
	g, err := NewRemessa(testCompany, Bank{Codigo: "001", VersaoArquivo: "084", VersaoLote: "042"}).
		AddBoleto(Boleto{NossoNumero: "1", NumeroDocumento: "X", Vencimento: d(2026, 1, 2), Valor: 1, Pagador: pagJoao, Aceite: true, DiasBaixa: 30, UsoEmpresa: "pedido 9"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	p = g.Titulos()[0].P
	if p.DataEmissao.IsZero() || p.Carteira != 1 || p.Aceite != "A" || p.CodigoBaixa != 1 || p.DiasBaixa != "030" || p.UsoEmpresa != "PEDIDO 9" {
		t.Errorf("defaults %+v", p)
	}
	if g.Header.VersaoLayout != "084" || g.Batches[0].Header.VersaoLayout != "042" {
		t.Error("layout versions not applied")
	}
}

func TestRemessaBuilderErrors(t *testing.T) {
	ok := Boleto{NossoNumero: "1", NumeroDocumento: "X", Vencimento: d(2026, 1, 2), Valor: 100, Pagador: pagJoao}
	mut := func(f func(*Boleto)) Boleto { b := ok; f(&b); return b }
	cases := map[string]Boleto{
		"nosso número": mut(func(b *Boleto) { b.NossoNumero = "" }),
		"nn long":      mut(func(b *Boleto) { b.NossoNumero = strings.Repeat("1", 21) }),
		"valor":        mut(func(b *Boleto) { b.Valor = 0 }),
		"vencimento":   mut(func(b *Boleto) { b.Vencimento = time.Time{} }),
		"documento":    mut(func(b *Boleto) { b.NumeroDocumento = "" }),
		"doc charset":  mut(func(b *Boleto) { b.NumeroDocumento = "€" }),
		"negative":     mut(func(b *Boleto) { b.Desconto = -1 }),
		"protesto":     mut(func(b *Boleto) { b.DiasProtesto = 100 }),
		"cpf":          mut(func(b *Boleto) { b.Pagador.Documento = "52998224724" }),
		"cep":          mut(func(b *Boleto) { b.Pagador.CEP = "2224000" }),
		"cep letters":  mut(func(b *Boleto) { b.Pagador.CEP = "2224000A" }),
		"nome":         mut(func(b *Boleto) { b.Pagador.Nome = "" }),
		"uf":           mut(func(b *Boleto) { b.Pagador.UF = "R" }),
		"pagador text": mut(func(b *Boleto) { b.Pagador.Cidade = "東京" }),
		"uso empresa":  mut(func(b *Boleto) { b.UsoEmpresa = "\x01" }),
		"mensagem":     mut(func(b *Boleto) { b.Mensagem = "\x01" }),
	}
	for name, b := range cases {
		_, err := NewRemessa(testCompany, testBank).AddBoleto(ok).AddBoleto(b).Build()
		if !errors.Is(err, ErrInvalidBoleto) || !strings.Contains(err.Error(), "boleto 2") {
			t.Errorf("%s: %v", name, err)
		}
	}
	builds := map[string]*Remessa{
		"empty":        NewRemessa(testCompany, testBank),
		"company doc":  NewRemessa(Company{Nome: "X", Documento: "1"}, testBank).AddBoleto(ok),
		"bank code":    NewRemessa(testCompany, Bank{Codigo: "34"}).AddBoleto(ok),
		"company name": NewRemessa(Company{Documento: testCompany.Documento}, testBank).AddBoleto(ok),
		"bank name":    NewRemessa(testCompany, Bank{Codigo: "341", Nome: "\x00"}).AddBoleto(ok),
		"agencia":      NewRemessa(Company{Nome: "X", Documento: testCompany.Documento, Agencia: "12a"}, testBank).AddBoleto(ok),
	}
	for name, r := range builds {
		if _, err := r.Build(); !errors.Is(err, ErrInvalidBoleto) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := NewRemessa(testCompany, Bank{}).WriteTo(&bytes.Buffer{}); err == nil {
		t.Error("WriteTo ignored build error")
	}
	// Generation time defaults to now.
	f, err := NewRemessa(testCompany, testBank).AddBoleto(ok).Build()
	if err != nil || f.Header.DataGeracao.IsZero() {
		t.Errorf("default time: %v", err)
	}
	// Long text is truncated, not rejected.
	long := mut(func(b *Boleto) { b.Pagador.Nome = strings.Repeat("Á", 50) })
	f, err = NewRemessa(testCompany, testBank).AddBoleto(long).Build()
	if err != nil || f.Titulos()[0].Q.Nome != strings.Repeat("A", 40) {
		t.Errorf("truncation: %v", err)
	}
}
