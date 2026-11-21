// Package cnab400 declares the skeleton shared by the classic 400-position
// CNAB layouts: a header (record type 0), detail records (type 1, and
// bank-specific types such as 2, 5 or 7) and a trailer (type 9), each
// ending with a six-digit sequence number at positions 395-400.
//
// Unlike CNAB 240, the 400 layouts were never standardized by FEBRABAN in
// practice: every bank defines positions 2-394 of the detail record itself.
// This package therefore types only the parts that are common to the
// mainstream cobrança layouts and keeps the rest raw. Decode details with a
// bank-specific struct, for example
// [github.com/victorcondackdev/cnab-go/cnab400/bradesco], or write your own
// struct from your bank's manual (see the fixedwidth package documentation).
package cnab400

import (
	"time"

	"github.com/victorcondackdev/cnab-go/fixedwidth"
)

// LineLength is the length in bytes of every CNAB 400 record.
const LineLength = 400

// Header is the generic cobrança file header (record type 0). Positions
// 27-46 (company code) and 101-394 are bank-specific and kept raw.
type Header struct {
	Registro        string    `cnab:"1,1,const=0"`
	Operacao        int       `cnab:"2,1,num"`    // 1 remessa, 2 retorno
	LiteralOperacao string    `cnab:"3,7,alfa"`   // "REMESSA" / "RETORNO"
	Servico         int       `cnab:"10,2,num"`   // 01 cobrança
	LiteralServico  string    `cnab:"12,15,alfa"` // "COBRANCA"
	CodigoEmpresa   string    `cnab:"27,20,raw"`
	NomeEmpresa     string    `cnab:"47,30,alfa"`
	Banco           string    `cnab:"77,3,num"`
	NomeBanco       string    `cnab:"80,15,alfa"`
	DataGravacao    time.Time `cnab:"95,6,date"` // DDMMYY
	Reservado       string    `cnab:"101,294,raw"`
	Sequencial      int       `cnab:"395,6,num"`
}

// Detail is a detail record with only the record type and sequence number
// typed. Use [Detail.Decode] to read it with a bank-specific struct.
type Detail struct {
	Registro   string `cnab:"1,1,raw"`
	Dados      string `cnab:"2,393,raw"`
	Sequencial int    `cnab:"395,6,num"`
}

// Line re-encodes the detail as a 400-byte line.
func (d *Detail) Line() (string, error) { return fixedwidth.Marshal(d, LineLength) }

// Decode decodes the detail into a bank-specific struct (a pointer to a
// struct whose layout covers positions 1-400).
func (d *Detail) Decode(v any) error {
	line, err := d.Line()
	if err != nil {
		return err
	}
	return fixedwidth.Unmarshal(line, v)
}

// Trailer is the file trailer (record type 9). Positions 2-394 are
// bank-specific (retorno trailers usually carry totals) and kept raw.
type Trailer struct {
	Registro   string `cnab:"1,1,const=9"`
	Reservado  string `cnab:"2,393,raw"`
	Sequencial int    `cnab:"395,6,num"`
}

// File is a parsed CNAB 400 file.
type File struct {
	Header  Header
	Details []Detail
	Trailer Trailer
}
