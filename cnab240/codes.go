package cnab240

import "fmt"

// MovimentoRemessa is the "código de movimento remessa" (positions 16-17 of
// segments P, Q and R): the instruction the company sends to the bank.
//
// Only the codes defined identically by the FEBRABAN generic layout are
// declared. Banks accept subsets and add their own codes.
type MovimentoRemessa int

// Movement codes for remessa files.
const (
	RemessaEntrada              MovimentoRemessa = 1  // entrada de títulos
	RemessaBaixa                MovimentoRemessa = 2  // pedido de baixa
	RemessaAbatimento           MovimentoRemessa = 4  // concessão de abatimento
	RemessaCancelaAbatimento    MovimentoRemessa = 5  // cancelamento de abatimento
	RemessaAlteracaoVencimento  MovimentoRemessa = 6  // alteração de vencimento
	RemessaDesconto             MovimentoRemessa = 7  // concessão de desconto
	RemessaCancelaDesconto      MovimentoRemessa = 8  // cancelamento de desconto
	RemessaProtestar            MovimentoRemessa = 9  // protestar
	RemessaSustarProtestoBaixar MovimentoRemessa = 10 // sustar protesto e baixar título
	RemessaSustarProtestoManter MovimentoRemessa = 11 // sustar protesto e manter em carteira
	RemessaAlteracaoOutrosDados MovimentoRemessa = 31 // alteração de outros dados
)

var remessaNames = map[MovimentoRemessa]string{
	RemessaEntrada:              "Entrada de títulos",
	RemessaBaixa:                "Pedido de baixa",
	RemessaAbatimento:           "Concessão de abatimento",
	RemessaCancelaAbatimento:    "Cancelamento de abatimento",
	RemessaAlteracaoVencimento:  "Alteração de vencimento",
	RemessaDesconto:             "Concessão de desconto",
	RemessaCancelaDesconto:      "Cancelamento de desconto",
	RemessaProtestar:            "Protestar",
	RemessaSustarProtestoBaixar: "Sustar protesto e baixar título",
	RemessaSustarProtestoManter: "Sustar protesto e manter em carteira",
	RemessaAlteracaoOutrosDados: "Alteração de outros dados",
}

// String returns the Portuguese description, or "Movimento NN" for codes
// not declared here.
func (m MovimentoRemessa) String() string {
	if s, ok := remessaNames[m]; ok {
		return s
	}
	return fmt.Sprintf("Movimento %02d", int(m))
}

// MovimentoRetorno is the "código de movimento retorno" (positions 16-17 of
// segments T and U): the occurrence the bank reports back.
//
// Only codes with the same meaning across the FEBRABAN generic layout are
// declared. Banks add their own codes; check the manual for anything not
// listed here.
type MovimentoRetorno int

// Movement codes for retorno files.
const (
	RetornoEntradaConfirmada         MovimentoRetorno = 2  // entrada confirmada
	RetornoEntradaRejeitada          MovimentoRetorno = 3  // entrada rejeitada
	RetornoLiquidacao                MovimentoRetorno = 6  // liquidação
	RetornoBaixa                     MovimentoRetorno = 9  // baixa
	RetornoConfirmacaoAbatimento     MovimentoRetorno = 12 // confirmação de recebimento de instrução de abatimento
	RetornoConfirmacaoCancAbatimento MovimentoRetorno = 13 // confirmação de cancelamento de abatimento
	RetornoConfirmacaoVencimento     MovimentoRetorno = 14 // confirmação de alteração de vencimento
	RetornoLiquidacaoAposBaixa       MovimentoRetorno = 17 // liquidação após baixa ou de título não registrado
	RetornoConfirmacaoProtesto       MovimentoRetorno = 19 // confirmação de recebimento de instrução de protesto
	RetornoConfirmacaoSustacao       MovimentoRetorno = 20 // confirmação de recebimento de instrução de sustação de protesto
	RetornoRemessaCartorio           MovimentoRetorno = 23 // remessa a cartório
	RetornoRetiradaCartorio          MovimentoRetorno = 24 // retirada de cartório e manutenção em carteira
	RetornoProtestadoBaixado         MovimentoRetorno = 25 // protestado e baixado
	RetornoInstrucaoRejeitada        MovimentoRetorno = 26 // instrução rejeitada
	RetornoConfirmacaoOutrosDados    MovimentoRetorno = 27 // confirmação do pedido de alteração de outros dados
	RetornoDebitoTarifas             MovimentoRetorno = 28 // débito de tarifas/custas
	RetornoAlteracaoRejeitada        MovimentoRetorno = 30 // alteração de dados rejeitada
)

var retornoNames = map[MovimentoRetorno]string{
	RetornoEntradaConfirmada:         "Entrada confirmada",
	RetornoEntradaRejeitada:          "Entrada rejeitada",
	RetornoLiquidacao:                "Liquidação",
	RetornoBaixa:                     "Baixa",
	RetornoConfirmacaoAbatimento:     "Confirmação de abatimento",
	RetornoConfirmacaoCancAbatimento: "Confirmação de cancelamento de abatimento",
	RetornoConfirmacaoVencimento:     "Confirmação de alteração de vencimento",
	RetornoLiquidacaoAposBaixa:       "Liquidação após baixa ou título não registrado",
	RetornoConfirmacaoProtesto:       "Confirmação de instrução de protesto",
	RetornoConfirmacaoSustacao:       "Confirmação de sustação de protesto",
	RetornoRemessaCartorio:           "Remessa a cartório",
	RetornoRetiradaCartorio:          "Retirada de cartório",
	RetornoProtestadoBaixado:         "Protestado e baixado",
	RetornoInstrucaoRejeitada:        "Instrução rejeitada",
	RetornoConfirmacaoOutrosDados:    "Confirmação de alteração de outros dados",
	RetornoDebitoTarifas:             "Débito de tarifas/custas",
	RetornoAlteracaoRejeitada:        "Alteração de dados rejeitada",
}

// String returns the Portuguese description, or "Movimento NN" for codes
// not declared here.
func (m MovimentoRetorno) String() string {
	if s, ok := retornoNames[m]; ok {
		return s
	}
	return fmt.Sprintf("Movimento %02d", int(m))
}

// IsLiquidacao reports whether the occurrence settles the title (06 or 17).
func (m MovimentoRetorno) IsLiquidacao() bool {
	return m == RetornoLiquidacao || m == RetornoLiquidacaoAposBaixa
}

// IsRejeicao reports whether the occurrence rejects an instruction (03, 26
// or 30).
func (m MovimentoRetorno) IsRejeicao() bool {
	return m == RetornoEntradaRejeitada || m == RetornoInstrucaoRejeitada || m == RetornoAlteracaoRejeitada
}

// Especie is the "espécie do título" (positions 107-108 of segment P).
// Only a few widely used codes are declared; the full table is in the
// FEBRABAN manual and banks restrict it further.
type Especie int

// Espécie codes.
const (
	EspecieDuplicataMercantil Especie = 2  // DM
	EspecieDuplicataServico   Especie = 4  // DS
	EspecieNotaPromissoria    Especie = 12 // NP
	EspecieRecibo             Especie = 17 // RC
	EspecieOutros             Especie = 99
)

// Juros (late interest) codes of segment P, position 118.
const (
	JurosValorDiario = 1 // valor por dia
	JurosTaxaMensal  = 2 // taxa mensal
	JurosIsento      = 3 // isento
)

// Desconto codes of segments P and R.
const (
	SemDesconto        = 0
	DescontoValorFixo  = 1 // valor fixo até a data informada
	DescontoPercentual = 2 // percentual até a data informada
)

// Multa (fine) codes of segment R, position 66.
const (
	SemMulta        = 0
	MultaValorFixo  = 1
	MultaPercentual = 2
)

// Protesto codes of segment P, position 221, and baixa codes, position 224.
const (
	ProtestarDiasCorridos = 1
	NaoProtestar          = 3
	BaixarDevolver        = 1
	NaoBaixar             = 2
)

// MoedaReal is the currency code for BRL (positions 228-229 of segment P).
const MoedaReal = 9
