package cnab240_test

import (
	"fmt"
	"time"

	"github.com/victorcondackdev/cnab-go/cnab240"
	"github.com/victorcondackdev/cnab-go/fixedwidth"
)

func Example_marshalSegmentP() {
	p := cnab240.SegmentP{
		DetalheControle: cnab240.DetalheControle{Controle: cnab240.Controle{Banco: "341", Lote: 1}, Sequencial: 1},
		Movimento:       cnab240.RemessaEntrada,
		Agencia:         "01234",
		Conta:           "000000056789",
		ContaDV:         "0",
		NossoNumero:     "00000001",
		Carteira:        1,
		NumeroDocumento: "NF-1001",
		Vencimento:      time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC),
		Valor:           150000, // cents: R$ 1.500,00
		Especie:         cnab240.EspecieDuplicataMercantil,
		Aceite:          "N",
		Moeda:           cnab240.MoedaReal,
	}
	line, err := fixedwidth.Marshal(p, cnab240.LineLength)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(len(line))
	fmt.Printf("controle    %q\n", line[0:17])
	fmt.Printf("nosso num.  %q\n", line[37:57])
	fmt.Printf("vencimento  %q\n", line[77:85])
	fmt.Printf("valor       %q\n", line[85:100])
	// Output:
	// 240
	// controle    "3410001300001P 01"
	// nosso num.  "00000001            "
	// vencimento  "10122026"
	// valor       "000000000150000"
}

func ExampleDecodeSegment() {
	t := &cnab240.SegmentT{Movimento: cnab240.RetornoEntradaRejeitada, NossoNumero: "123", Motivos: "0846"}
	line, _ := cnab240.EncodeSegment(t)
	s, err := cnab240.DecodeSegment(line)
	if err != nil {
		fmt.Println(err)
		return
	}
	got := s.(*cnab240.SegmentT)
	fmt.Println(got.Movimento, got.MotivoCodes())
	// Output: Entrada rejeitada [08 46]
}
