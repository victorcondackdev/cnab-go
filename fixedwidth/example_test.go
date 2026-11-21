package fixedwidth_test

import (
	"errors"
	"fmt"
	"time"

	"github.com/victorcondackdev/cnab-go/fixedwidth"
)

// A record is just a struct. Positions are copied from the bank manual.
type Pagamento struct {
	Tipo  string    `cnab:"1,1,const=1"`
	Nome  string    `cnab:"2,15,alfa"`
	Data  time.Time `cnab:"17-24,date"`
	Valor int64     `cnab:"25,10,money"`
}

func Example() {
	line, err := fixedwidth.Marshal(Pagamento{Nome: "José Conceição", Data: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC), Valor: 123456}, 34)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%q\n", line)

	var p Pagamento
	if err := fixedwidth.Unmarshal(line, &p); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(p.Nome, p.Data.Format(time.DateOnly), p.Valor)
	// Output:
	// "1JOSE CONCEICAO 050320260000123456"
	// JOSE CONCEICAO 2026-03-05 123456
}

// A typo in a layout is reported the first time the type is used.
type Typo struct {
	Banco string `cnab:"1,3,num"`
	Lote  string `cnab:"3,4,num"` // should start at 4
	Tipo  string `cnab:"8,1,alfa"`
}

func ExampleCheck() {
	err := fixedwidth.Check(Typo{}, 8)
	fmt.Println(errors.Is(err, fixedwidth.ErrLayout))
	fmt.Println(err)
	// Output:
	// true
	// fixedwidth: invalid layout fixedwidth_test.Typo: overlap: Banco (1-3) and Lote (3-6); gap at 7-7 between Lote and Tipo
}

func ExampleFieldError() {
	var p Pagamento
	err := fixedwidth.Unmarshal("1JOSE           3102202600001234X6", &p)
	var fe *fixedwidth.FieldError
	if errors.As(err, &fe) {
		fmt.Println(fe.Field, fe.Start, fe.End, errors.Is(err, fixedwidth.ErrDate))
	}
	fmt.Println(err)
	// Output:
	// Data 17 24 true
	// cnab: field Data (17-24) value "31022026": not a valid date
}
