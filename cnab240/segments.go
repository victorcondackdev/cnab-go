package cnab240

import "time"

// Segment is a detail record (type 3, or a raw type 2/4 record) of a batch.
// The concrete types are [*SegmentP], [*SegmentQ], [*SegmentR],
// [*SegmentT], [*SegmentU] and [*RawSegment].
type Segment interface {
	// Code returns the segment code at position 14 ('P', 'Q', ...), or 0 for
	// raw records that are not type 3.
	Code() byte
}

// SegmentP carries the title data of a remessa ("segmento P", mandatory).
//
// NossoNumero (38-57) is where banks diverge the most: each bank defines
// its own sub-layout and alignment (left-aligned with spaces or right-aligned
// with zeros, with or without check digit). It is kept as raw text.
type SegmentP struct {
	DetalheControle
	Segmento           string           `cnab:"14,1,const=P"`
	CNAB1              string           `cnab:"15,1,alfa"`
	Movimento          MovimentoRemessa `cnab:"16,2,num"`
	Agencia            string           `cnab:"18,5,num"`
	AgenciaDV          string           `cnab:"23,1,alfa"`
	Conta              string           `cnab:"24,12,num"`
	ContaDV            string           `cnab:"36,1,alfa"`
	AgenciaContaDV     string           `cnab:"37,1,alfa"`
	NossoNumero        string           `cnab:"38,20,alfa"`
	Carteira           int              `cnab:"58,1,num"`
	FormaCadastramento int              `cnab:"59,1,num"`
	TipoDocumento      string           `cnab:"60,1,alfa"`
	EmissaoBoleto      int              `cnab:"61,1,num"`
	DistribuicaoBoleto string           `cnab:"62,1,alfa"`
	NumeroDocumento    string           `cnab:"63,15,alfa"` // "seu número"
	Vencimento         time.Time        `cnab:"78,8,date"`
	Valor              int64            `cnab:"86,15,money"`
	AgenciaCobradora   string           `cnab:"101,5,num"`
	AgenciaCobradoraDV string           `cnab:"106,1,alfa"`
	Especie            Especie          `cnab:"107,2,num"`
	Aceite             string           `cnab:"109,1,alfa"` // A or N
	DataEmissao        time.Time        `cnab:"110,8,date"`
	CodigoJuros        int              `cnab:"118,1,num"`
	DataJuros          time.Time        `cnab:"119,8,date"`
	Juros              int64            `cnab:"127,15,money"` // per day (value) or rate, per CodigoJuros
	CodigoDesconto     int              `cnab:"142,1,num"`
	DataDesconto       time.Time        `cnab:"143,8,date"`
	Desconto           int64            `cnab:"151,15,money"`
	IOF                int64            `cnab:"166,15,money"`
	Abatimento         int64            `cnab:"181,15,money"`
	UsoEmpresa         string           `cnab:"196,25,alfa"` // identificação do título na empresa
	CodigoProtesto     int              `cnab:"221,1,num"`
	DiasProtesto       int              `cnab:"222,2,num"`
	CodigoBaixa        int              `cnab:"224,1,num"`
	DiasBaixa          string           `cnab:"225,3,alfa"` // numeric in the manual, often blank
	Moeda              int              `cnab:"228,2,num"`
	NumeroContrato     string           `cnab:"230,10,num"`
	UsoLivre           string           `cnab:"240,1,alfa"`
}

// Code implements [Segment].
func (*SegmentP) Code() byte { return 'P' }

// SegmentQ carries payer ("pagador") and guarantor ("sacador/avalista")
// data of a remessa (mandatory after P).
type SegmentQ struct {
	DetalheControle
	Segmento              string           `cnab:"14,1,const=Q"`
	CNAB1                 string           `cnab:"15,1,alfa"`
	Movimento             MovimentoRemessa `cnab:"16,2,num"`
	TipoInscricao         int              `cnab:"18,1,num"`
	NumeroInscricao       string           `cnab:"19,15,num"`
	Nome                  string           `cnab:"34,40,alfa"`
	Endereco              string           `cnab:"74,40,alfa"`
	Bairro                string           `cnab:"114,15,alfa"`
	CEP                   string           `cnab:"129,5,num"`
	SufixoCEP             string           `cnab:"134,3,num"`
	Cidade                string           `cnab:"137,15,alfa"`
	UF                    string           `cnab:"152,2,alfa"`
	TipoInscricaoAvalista int              `cnab:"154,1,num"`
	InscricaoAvalista     string           `cnab:"155,15,num"`
	NomeAvalista          string           `cnab:"170,40,alfa"`
	BancoCorrespondente   string           `cnab:"210,3,num"`
	NossoNumeroCorresp    string           `cnab:"213,20,alfa"`
	CNAB2                 string           `cnab:"233,8,alfa"`
}

// Code implements [Segment].
func (*SegmentQ) Code() byte { return 'Q' }

// SegmentR carries optional data of a remessa: second and third discounts,
// fine ("multa"), messages and automatic debit data.
type SegmentR struct {
	DetalheControle
	Segmento          string           `cnab:"14,1,const=R"`
	CNAB1             string           `cnab:"15,1,alfa"`
	Movimento         MovimentoRemessa `cnab:"16,2,num"`
	CodigoDesconto2   int              `cnab:"18,1,num"`
	DataDesconto2     time.Time        `cnab:"19,8,date"`
	Desconto2         int64            `cnab:"27,15,money"`
	CodigoDesconto3   int              `cnab:"42,1,num"`
	DataDesconto3     time.Time        `cnab:"43,8,date"`
	Desconto3         int64            `cnab:"51,15,money"`
	CodigoMulta       int              `cnab:"66,1,num"`
	DataMulta         time.Time        `cnab:"67,8,date"`
	Multa             int64            `cnab:"75,15,money"` // value or percentage, per CodigoMulta
	InformacaoPagador string           `cnab:"90,10,alfa"`
	Mensagem3         string           `cnab:"100,40,alfa"`
	Mensagem4         string           `cnab:"140,40,alfa"`
	CNAB2             string           `cnab:"180,20,alfa"`
	OcorrenciaPagador string           `cnab:"200,8,alfa"`
	BancoDebito       string           `cnab:"208,3,num"`
	AgenciaDebito     string           `cnab:"211,5,num"`
	AgenciaDebitoDV   string           `cnab:"216,1,alfa"`
	ContaDebito       string           `cnab:"217,12,num"`
	ContaDebitoDV     string           `cnab:"229,1,alfa"`
	AgContaDebitoDV   string           `cnab:"230,1,alfa"`
	AvisoDebito       string           `cnab:"231,1,alfa"`
	CNAB3             string           `cnab:"232,9,alfa"`
}

// Code implements [Segment].
func (*SegmentR) Code() byte { return 'R' }

// SegmentT carries the title data of a retorno ("segmento T").
//
// Motivos (214-223) holds up to five two-character reason codes whose
// meaning depends on Movimento and, in practice, on the bank. Use
// [SegmentT.MotivoCodes] to split them.
type SegmentT struct {
	DetalheControle
	Segmento           string           `cnab:"14,1,const=T"`
	CNAB1              string           `cnab:"15,1,alfa"`
	Movimento          MovimentoRetorno `cnab:"16,2,num"`
	Agencia            string           `cnab:"18,5,num"`
	AgenciaDV          string           `cnab:"23,1,alfa"`
	Conta              string           `cnab:"24,12,num"`
	ContaDV            string           `cnab:"36,1,alfa"`
	AgenciaContaDV     string           `cnab:"37,1,alfa"`
	NossoNumero        string           `cnab:"38,20,alfa"`
	Carteira           int              `cnab:"58,1,num"`
	NumeroDocumento    string           `cnab:"59,15,alfa"`
	Vencimento         time.Time        `cnab:"74,8,date"`
	Valor              int64            `cnab:"82,15,money"`
	BancoCobrador      string           `cnab:"97,3,num"`
	AgenciaCobradora   string           `cnab:"100,5,num"`
	AgenciaCobradoraDV string           `cnab:"105,1,alfa"`
	UsoEmpresa         string           `cnab:"106,25,alfa"`
	Moeda              int              `cnab:"131,2,num"`
	TipoInscricao      int              `cnab:"133,1,num"`
	NumeroInscricao    string           `cnab:"134,15,num"`
	Nome               string           `cnab:"149,40,alfa"`
	NumeroContrato     string           `cnab:"189,10,num"`
	Tarifa             int64            `cnab:"199,15,money"` // valor da tarifa / custas
	Motivos            string           `cnab:"214,10,alfa"`
	CNAB2              string           `cnab:"224,17,alfa"`
}

// Code implements [Segment].
func (*SegmentT) Code() byte { return 'T' }

// MotivoCodes splits Motivos into its non-blank, non-"00" two-character
// codes.
func (t *SegmentT) MotivoCodes() []string {
	var out []string
	m := t.Motivos
	for i := 0; i+2 <= len(m); i += 2 {
		if c := m[i : i+2]; c != "  " && c != "00" {
			out = append(out, c)
		}
	}
	if len(m)%2 == 1 && m[len(m)-1] != ' ' {
		out = append(out, m[len(m)-1:])
	}
	return out
}

// SegmentU carries the amounts and dates of a retorno occurrence
// ("segmento U", follows T).
//
// ValorPago is what the payer paid; ValorLiquido is what will be credited
// (banks differ on whether tariffs are already deducted).
type SegmentU struct {
	DetalheControle
	Segmento               string           `cnab:"14,1,const=U"`
	CNAB1                  string           `cnab:"15,1,alfa"`
	Movimento              MovimentoRetorno `cnab:"16,2,num"`
	JurosMulta             int64            `cnab:"18,15,money"` // juros/multa/encargos
	Desconto               int64            `cnab:"33,15,money"`
	Abatimento             int64            `cnab:"48,15,money"`
	IOF                    int64            `cnab:"63,15,money"`
	ValorPago              int64            `cnab:"78,15,money"`
	ValorLiquido           int64            `cnab:"93,15,money"`
	OutrasDespesas         int64            `cnab:"108,15,money"`
	OutrosCreditos         int64            `cnab:"123,15,money"`
	DataOcorrencia         time.Time        `cnab:"138,8,date"`
	DataCredito            time.Time        `cnab:"146,8,date"`
	OcorrenciaPagador      string           `cnab:"154,4,alfa"`
	DataOcorrenciaPagador  time.Time        `cnab:"158,8,date"`
	ValorOcorrenciaPagador int64            `cnab:"166,15,money"`
	ComplementoOcorrencia  string           `cnab:"181,30,alfa"`
	BancoCorrespondente    string           `cnab:"211,3,num"`
	NossoNumeroCorresp     string           `cnab:"214,20,alfa"`
	CNAB2                  string           `cnab:"234,7,alfa"`
}

// Code implements [Segment].
func (*SegmentU) Code() byte { return 'U' }

// RawSegment keeps a batch record this package has no struct for (other
// segments such as A, J, S, Y, or record types 2 and 4) verbatim, so files
// round-trip losslessly. Line has no terminator.
type RawSegment struct {
	Line string
}

// Code implements [Segment]: the byte at position 14 for type 3 records,
// 0 otherwise.
func (r *RawSegment) Code() byte {
	if len(r.Line) >= 14 && r.Line[7] == RegistroDetalhe {
		return r.Line[13]
	}
	return 0
}
