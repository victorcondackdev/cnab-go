// Package bradesco declares the CNAB 400 cobrança remessa detail record
// (type 1) of Banco Bradesco (237), as an example of a bank-specific
// layout built on [github.com/victorcondackdev/cnab-go/fixedwidth].
//
// EXPERIMENTAL: the positions follow the publicly circulated Bradesco
// "Cobrança Bancária – CNAB 400" remessa manual. They have not been
// homologated against the bank; validate them with your account manager
// before production use. The retorno layout is not declared yet.
package bradesco

import "time"

// Banco is Bradesco's compensation code.
const Banco = "237"

// DetalheRemessa is the Bradesco CNAB 400 remessa detail record (type 1).
type DetalheRemessa struct {
	Registro             string    `cnab:"1,1,const=1"`
	AgenciaDebito        string    `cnab:"2,5,num"`
	AgenciaDebitoDV      string    `cnab:"7,1,alfa"`
	RazaoContaDebito     string    `cnab:"8,5,num"`
	ContaDebito          string    `cnab:"13,7,num"`
	ContaDebitoDV        string    `cnab:"20,1,alfa"`
	IdentificacaoEmpresa string    `cnab:"21,17,alfa"` // 0 + carteira(3) + agência(5) + conta(7) + DV
	ControleParticipante string    `cnab:"38,25,alfa"`
	BancoDebito          string    `cnab:"63,3,num"`
	CampoMulta           int       `cnab:"66,1,num"` // 0 sem multa, 2 com multa
	PercentualMulta      int       `cnab:"67,4,num"` // 2 implied decimals
	NossoNumero          string    `cnab:"71,11,num"`
	NossoNumeroDV        string    `cnab:"82,1,alfa"`
	DescontoDia          int64     `cnab:"83,10,money"`
	CondicaoEmissao      int       `cnab:"93,1,num"` // 1 banco emite, 2 cliente emite
	DebitoAutomatico     string    `cnab:"94,1,alfa"`
	OperacaoBanco        string    `cnab:"95,10,alfa"`
	IndicadorRateio      string    `cnab:"105,1,alfa"`
	AvisoDebito          string    `cnab:"106,1,alfa"`
	QuantidadePagamentos string    `cnab:"107,2,alfa"`
	Ocorrencia           int       `cnab:"109,2,num"` // 01 remessa, 02 pedido de baixa, ...
	NumeroDocumento      string    `cnab:"111,10,alfa"`
	Vencimento           time.Time `cnab:"121,6,date"`
	Valor                int64     `cnab:"127,13,money"`
	BancoCobranca        string    `cnab:"140,3,num"`
	AgenciaDepositaria   string    `cnab:"143,5,num"`
	Especie              int       `cnab:"148,2,num"`
	Identificacao        string    `cnab:"150,1,alfa"` // always N
	DataEmissao          time.Time `cnab:"151,6,date"`
	Instrucao1           int       `cnab:"157,2,num"`
	Instrucao2           int       `cnab:"159,2,num"`
	MoraDia              int64     `cnab:"161,13,money"`
	DataLimiteDesconto   time.Time `cnab:"174,6,date"`
	Desconto             int64     `cnab:"180,13,money"`
	IOF                  int64     `cnab:"193,13,money"`
	Abatimento           int64     `cnab:"206,13,money"`
	TipoInscricao        int       `cnab:"219,2,num"` // 01 CPF, 02 CNPJ
	NumeroInscricao      string    `cnab:"221,14,num"`
	Nome                 string    `cnab:"235,40,alfa"`
	Endereco             string    `cnab:"275,40,alfa"`
	Mensagem1            string    `cnab:"315,12,alfa"`
	CEP                  string    `cnab:"327,5,num"`
	SufixoCEP            string    `cnab:"332,3,num"`
	SacadorOuMensagem2   string    `cnab:"335,60,alfa"`
	Sequencial           int       `cnab:"395,6,num"`
}
