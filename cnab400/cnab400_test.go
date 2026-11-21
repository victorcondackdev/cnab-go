package cnab400_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/victorcondackdev/cnab-go/cnab400"
	"github.com/victorcondackdev/cnab-go/cnab400/bradesco"
	"github.com/victorcondackdev/cnab-go/fixedwidth"
)

func TestLayouts(t *testing.T) {
	for _, v := range []any{cnab400.Header{}, cnab400.Detail{}, cnab400.Trailer{}, bradesco.DetalheRemessa{}} {
		if err := fixedwidth.Check(v, cnab400.LineLength); err != nil {
			t.Errorf("%T: %v", v, err)
		}
	}
}

func TestBradescoDetailRoundTrip(t *testing.T) {
	want := bradesco.DetalheRemessa{
		Registro:             "1",
		AgenciaDebito:        "00000",
		AgenciaDebitoDV:      "0",
		RazaoContaDebito:     "00000",
		ContaDebito:          "0000000",
		ContaDebitoDV:        "0",
		IdentificacaoEmpresa: "00090123400123456",
		ControleParticipante: "PEDIDO 1001",
		BancoDebito:          "000",
		CampoMulta:           2,
		PercentualMulta:      200,
		NossoNumero:          "00000000123",
		NossoNumeroDV:        "P",
		CondicaoEmissao:      2,
		DebitoAutomatico:     "N",
		Ocorrencia:           1,
		NumeroDocumento:      "NF1001",
		Vencimento:           time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC),
		Valor:                150000,
		BancoCobranca:        "000",
		AgenciaDepositaria:   "00000",
		Especie:              1,
		Identificacao:        "N",
		DataEmissao:          time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC),
		MoraDia:              50,
		TipoInscricao:        1,
		NumeroInscricao:      "00052998224725",
		Nome:                 "MARIA DA SILVA",
		Endereco:             "RUA DAS FLORES 100",
		CEP:                  "20000",
		SufixoCEP:            "000",
		Sequencial:           2,
	}
	line, err := fixedwidth.Marshal(want, cnab400.LineLength)
	if err != nil {
		t.Fatal(err)
	}
	if got := line[120:126]; got != "101226" {
		t.Errorf("vencimento at 121-126 = %q", got)
	}
	if got := line[126:139]; got != "0000000150000" {
		t.Errorf("valor at 127-139 = %q", got)
	}
	d := cnab400.Detail{}
	if err := fixedwidth.Unmarshal(line, &d); err != nil {
		t.Fatal(err)
	}
	var got bradesco.DetalheRemessa
	if err := d.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip:\n got %+v\nwant %+v", got, want)
	}
}

func TestDetailDecodeErrors(t *testing.T) {
	d := cnab400.Detail{Registro: "1", Dados: "x\n"}
	if err := d.Decode(&bradesco.DetalheRemessa{}); err == nil {
		t.Error("newline accepted")
	}
	d.Registro, d.Dados = "7", ""
	if err := d.Decode(&bradesco.DetalheRemessa{}); err == nil {
		t.Error("type 7 record decoded as a type 1 detail")
	}
}
