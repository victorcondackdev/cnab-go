package cnab240

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/victorcondackdev/cnab-go/fixedwidth"
)

func d(y, m, dd int) time.Time { return time.Date(y, time.Month(m), dd, 0, 0, 0, 0, time.UTC) }

func ctl(seq int) DetalheControle {
	return DetalheControle{Controle: Controle{Banco: "341", Lote: 1}, Registro: "3", Sequencial: seq}
}

func TestSegmentRoundTrip(t *testing.T) {
	segs := []Segment{
		&SegmentP{DetalheControle: ctl(1), Segmento: "P", Movimento: RemessaEntrada, Agencia: "01234", Conta: "000000056789", ContaDV: "0",
			NossoNumero: "00000001", Carteira: 1, FormaCadastramento: 1, TipoDocumento: "1", EmissaoBoleto: 2, DistribuicaoBoleto: "2",
			NumeroDocumento: "NF-1001", Vencimento: d(2026, 12, 10), Valor: 150000, AgenciaCobradora: "00000", Especie: EspecieDuplicataMercantil,
			Aceite: "N", DataEmissao: d(2026, 11, 20), CodigoJuros: JurosValorDiario, DataJuros: d(2026, 12, 11), Juros: 50,
			CodigoProtesto: NaoProtestar, Moeda: MoedaReal, NumeroContrato: "0000000000"},
		&SegmentQ{DetalheControle: ctl(2), Segmento: "Q", Movimento: RemessaEntrada, TipoInscricao: InscricaoCPF, NumeroInscricao: "000052998224725",
			Nome: "JOAO DA SILVA", Endereco: "RUA A, 1", Bairro: "CENTRO", CEP: "20000", SufixoCEP: "000", Cidade: "RIO DE JANEIRO", UF: "RJ",
			InscricaoAvalista: "000000000000000", BancoCorrespondente: "000"},
		&SegmentR{DetalheControle: ctl(3), Segmento: "R", Movimento: RemessaEntrada, CodigoMulta: MultaPercentual, DataMulta: d(2026, 12, 11), Multa: 200,
			Mensagem3: "OBRIGADO", BancoDebito: "000", AgenciaDebito: "00000", ContaDebito: "000000000000"},
		&SegmentT{DetalheControle: ctl(4), Segmento: "T", Movimento: RetornoLiquidacao, Agencia: "01234", Conta: "000000056789", NossoNumero: "00000001",
			Carteira: 1, NumeroDocumento: "NF-1001", Vencimento: d(2026, 12, 10), Valor: 150000, BancoCobrador: "341", AgenciaCobradora: "00500",
			Moeda: 9, TipoInscricao: 1, NumeroInscricao: "000052998224725", Nome: "JOAO", NumeroContrato: "0000000000", Tarifa: 350, Motivos: "00"},
		&SegmentU{DetalheControle: ctl(5), Segmento: "U", Movimento: RetornoLiquidacao, ValorPago: 150000, ValorLiquido: 149650,
			DataOcorrencia: d(2026, 12, 10), DataCredito: d(2026, 12, 11), BancoCorrespondente: "000"},
	}
	for _, s := range segs {
		line, err := EncodeSegment(s)
		if err != nil {
			t.Fatalf("%T: %v", s, err)
		}
		if len(line) != LineLength || line[13] != s.Code() {
			t.Fatalf("%T: bad line %q", s, line)
		}
		got, err := DecodeSegment(line)
		if err != nil {
			t.Fatalf("%T: %v", s, err)
		}
		if !reflect.DeepEqual(got, s) {
			t.Errorf("%T round trip:\n got %+v\nwant %+v", s, got, s)
		}
	}
}

func TestDecodeSegmentErrors(t *testing.T) {
	if _, err := DecodeSegment("x"); !errors.Is(err, fixedwidth.ErrLineLength) {
		t.Errorf("short: %v", err)
	}
	line, _ := EncodeSegment(&SegmentP{})
	bad := line[:85] + "X" + line[86:]
	if _, err := DecodeSegment(bad); !errors.Is(err, fixedwidth.ErrDigits) {
		t.Errorf("bad value: %v", err)
	}
	raw := strings.Repeat(" ", 240)
	s, err := DecodeSegment(raw)
	if err != nil || s.Code() != 0 {
		t.Errorf("raw: %v %v", s, err)
	}
	if _, err := EncodeSegment(&RawSegment{Line: "x"}); !errors.Is(err, ErrRawLength) {
		t.Errorf("raw length: %v", err)
	}
	if (&RawSegment{Line: "x"}).Code() != 0 {
		t.Error("short raw code")
	}
}

func TestGrouping(t *testing.T) {
	p1, p2 := &SegmentP{NossoNumero: "1"}, &SegmentP{NossoNumero: "2"}
	q1, r1, q2 := &SegmentQ{Nome: "A"}, &SegmentR{}, &SegmentQ{Nome: "B"}
	t1, u1, t2 := &SegmentT{NossoNumero: "1"}, &SegmentU{ValorPago: 1}, &SegmentT{NossoNumero: "2"}
	b := &Batch{Segments: []Segment{&SegmentQ{}, &SegmentU{}, p1, q1, r1, &SegmentQ{Nome: "dup"}, p2, q2, t1, u1, &SegmentU{}, t2, &RawSegment{}}}
	ts := b.Titulos()
	if len(ts) != 2 || ts[0] != (Titulo{p1, q1, r1}) || ts[1] != (Titulo{p2, q2, nil}) {
		t.Errorf("Titulos = %+v", ts)
	}
	os := b.Ocorrencias()
	if len(os) != 2 || os[0] != (Ocorrencia{t1, u1}) || os[1] != (Ocorrencia{t2, nil}) {
		t.Errorf("Ocorrencias = %+v", os)
	}
	f := &File{Batches: []*Batch{b, b}}
	if len(f.Titulos()) != 4 || len(f.Ocorrencias()) != 4 {
		t.Error("file-level grouping")
	}
}

func TestCodes(t *testing.T) {
	if RemessaEntrada.String() != "Entrada de títulos" || MovimentoRemessa(77).String() != "Movimento 77" {
		t.Error("MovimentoRemessa.String")
	}
	if RetornoLiquidacao.String() != "Liquidação" || MovimentoRetorno(5).String() != "Movimento 05" {
		t.Error("MovimentoRetorno.String")
	}
	for _, m := range []MovimentoRetorno{6, 17} {
		if !m.IsLiquidacao() || m.IsRejeicao() {
			t.Errorf("%d", m)
		}
	}
	for _, m := range []MovimentoRetorno{3, 26, 30} {
		if !m.IsRejeicao() || m.IsLiquidacao() {
			t.Errorf("%d", m)
		}
	}
	tests := map[string][]string{"": nil, "00": nil, "0846": {"08", "46"}, "08  46": {"08", "46"}, "A1B": {"A1", "B"}, "0000000000": nil}
	for in, want := range tests {
		if got := (&SegmentT{Motivos: in}).MotivoCodes(); !reflect.DeepEqual(got, want) {
			t.Errorf("MotivoCodes(%q) = %v, want %v", in, got, want)
		}
	}
}
