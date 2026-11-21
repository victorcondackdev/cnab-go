package file_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/victorcondackdev/cnab-go/file"
	"github.com/victorcondackdev/cnab-go/reconcile"
)

func ExampleNewRemessa() {
	company := file.Company{Nome: "Acme Utilidades Ltda", Documento: "11.222.333/0001-81", Agencia: "01234", Conta: "000000056789", ContaDV: "0"}
	bank := file.Bank{Codigo: "341", Nome: "Banco Itaú S.A."}
	r := file.NewRemessa(company, bank).
		WithNSA(42).
		WithGeneratedAt(time.Date(2026, 11, 20, 9, 30, 0, 0, time.UTC)).
		AddBoleto(file.Boleto{
			NossoNumero:     "00000001",
			NumeroDocumento: "NF-1001",
			Vencimento:      time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC),
			Valor:           150000, // R$ 1.500,00
			JurosPorDia:     50,
			Pagador: file.Pagador{Nome: "João da Silva", Documento: "529.982.247-25",
				Endereco: "Rua das Laranjeiras, 1500", Bairro: "Laranjeiras",
				CEP: "22240-003", Cidade: "Rio de Janeiro", UF: "RJ"},
		})
	f, err := r.Build()
	if err != nil {
		fmt.Println(err)
		return
	}
	var sb strings.Builder
	if err := file.Write(&sb, f); err != nil {
		fmt.Println(err)
		return
	}
	for _, line := range strings.Split(strings.TrimSuffix(sb.String(), "\r\n"), "\r\n") {
		fmt.Println(line[:16], len(line))
	}
	// Output:
	// 34100000         240
	// 34100011R01  060 240
	// 3410001300001P 0 240
	// 3410001300002Q 0 240
	// 34100015         240
	// 34199999         240
}

func ExampleParse() {
	in, err := os.Open("testdata/retorno_341.ret")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer in.Close()
	f, err := file.Parse(in)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, o := range f.Ocorrencias()[3:6] {
		fmt.Printf("%s %02d %s pago=%d\n", o.T.NossoNumero, int(o.T.Movimento), o.T.Movimento, o.U.ValorPago)
	}
	// Output:
	// 00000001 06 Liquidação pago=150000
	// 00000002 06 Liquidação pago=91880
	// 00000004 06 Liquidação pago=40000
}

func Example_reconcile() {
	in, err := os.Open("testdata/retorno_341.ret")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer in.Close()
	f, err := file.Parse(in)
	if err != nil {
		fmt.Println(err)
		return
	}
	due := func(day int) time.Time { return time.Date(2026, 12, day, 0, 0, 0, 0, time.UTC) }
	expected := []reconcile.Expected{
		{NossoNumero: "1", Valor: 150000, Vencimento: due(10)},
		{NossoNumero: "2", Valor: 89990, Vencimento: due(15)},
		{NossoNumero: "3", Valor: 25000, Vencimento: due(20)},
		{NossoNumero: "4", Valor: 42050, Vencimento: due(20)},
		{NossoNumero: "5", Valor: 10000, Vencimento: due(30)},
	}
	rep, err := reconcile.Reconcile(expected, reconcile.EntriesFromFile(f))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Print(rep.Summary())
	// Output:
	// Reconciliation: 9 retorno entries, 5 expected boletos
	//   paid exact                1
	//   paid with adjustment      1
	//   paid late                 1
	//   divergent                 0
	//   duplicate                 1
	//   unknown nosso número      1
	//   rejected                  1
	//   open                      2
	//   informational             3
	// Expected R$ 3.170,40, paid R$ 2.818,80, credited R$ 4.351,30, tariffs R$ 21,00
	// Attention:
	//   [rejected] 00000003: Entrada rejeitada; motivos 08, 46 (see the bank's manual)
	//   [paid_late] 00000002: paid on 2026-12-17, 2 day(s) after due date 2026-12-15; paid R$ 918,80, expected R$ 899,90 (difference R$ 18,90) explained by juros/multa R$ 18,90
	//   [unknown] 00000099: settlement of R$ 50,00 for a nosso número that is not expected
	//   [duplicate] 00000001: already settled on 2026-12-10 for R$ 1.500,00; this settlement of R$ 1.500,00 is a duplicate
}
