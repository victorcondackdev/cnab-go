package cnab240

import "time"

// LineLength is the length in bytes of every CNAB 240 record.
const LineLength = 240

// Record type codes (position 8, "tipo de registro").
const (
	RegistroHeaderArquivo  = '0'
	RegistroHeaderLote     = '1'
	RegistroIniciaisLote   = '2'
	RegistroDetalhe        = '3'
	RegistroFinaisLote     = '4'
	RegistroTrailerLote    = '5'
	RegistroTrailerArquivo = '9'
)

// Tipo de inscrição (company or payer document type).
const (
	InscricaoIsento = 0
	InscricaoCPF    = 1
	InscricaoCNPJ   = 2
)

// Código remessa/retorno of the file header (position 143).
const (
	CodigoRemessa = 1
	CodigoRetorno = 2
)

// Controle holds positions 1-7, shared by every record: bank code
// ("código do banco na compensação") and batch number ("lote de serviço";
// 0000 for the file header, 9999 for the file trailer).
type Controle struct {
	Banco string `cnab:"1,3,num"`
	Lote  int    `cnab:"4,4,num"`
}

// FileHeader is the file header, record type 0 ("header de arquivo").
//
// VersaoLayout (164-166) is bank- and version-specific: check the manual.
// ReservadoBanco (172-191) is used by several banks for their own data.
type FileHeader struct {
	Controle
	Registro             string    `cnab:"8,1,const=0"`
	CNAB1                string    `cnab:"9,9,alfa"`
	TipoInscricao        int       `cnab:"18,1,num"`
	NumeroInscricao      string    `cnab:"19,14,num"`
	Convenio             string    `cnab:"33,20,alfa"`
	Agencia              string    `cnab:"53,5,num"`
	AgenciaDV            string    `cnab:"58,1,alfa"`
	Conta                string    `cnab:"59,12,num"`
	ContaDV              string    `cnab:"71,1,alfa"`
	AgenciaContaDV       string    `cnab:"72,1,alfa"`
	NomeEmpresa          string    `cnab:"73,30,alfa"`
	NomeBanco            string    `cnab:"103,30,alfa"`
	CNAB2                string    `cnab:"133,10,alfa"`
	CodigoRemessaRetorno int       `cnab:"143,1,num"`
	DataGeracao          time.Time `cnab:"144,8,date"`
	HoraGeracao          string    `cnab:"152,6,num"` // HHMMSS
	NSA                  int       `cnab:"158,6,num"` // número sequencial do arquivo
	VersaoLayout         string    `cnab:"164,3,num"`
	Densidade            string    `cnab:"167,5,num"`
	ReservadoBanco       string    `cnab:"172,20,alfa"`
	ReservadoEmpresa     string    `cnab:"192,20,alfa"`
	CNAB3                string    `cnab:"212,29,alfa"`
}

// BatchHeader is the cobrança batch header, record type 1 ("header de
// lote"). Operacao is "R" for remessa and "T" for retorno; Servico is 01
// (cobrança). Note that the company inscription has 15 digits here, one more
// than in the file header.
type BatchHeader struct {
	Controle
	Registro        string    `cnab:"8,1,const=1"`
	Operacao        string    `cnab:"9,1,alfa"`
	Servico         int       `cnab:"10,2,num"`
	CNAB1           string    `cnab:"12,2,alfa"`
	VersaoLayout    string    `cnab:"14,3,num"`
	CNAB2           string    `cnab:"17,1,alfa"`
	TipoInscricao   int       `cnab:"18,1,num"`
	NumeroInscricao string    `cnab:"19,15,num"`
	Convenio        string    `cnab:"34,20,alfa"`
	Agencia         string    `cnab:"54,5,num"`
	AgenciaDV       string    `cnab:"59,1,alfa"`
	Conta           string    `cnab:"60,12,num"`
	ContaDV         string    `cnab:"72,1,alfa"`
	AgenciaContaDV  string    `cnab:"73,1,alfa"`
	NomeEmpresa     string    `cnab:"74,30,alfa"`
	Mensagem1       string    `cnab:"104,40,alfa"`
	Mensagem2       string    `cnab:"144,40,alfa"`
	NumeroRemessa   int       `cnab:"184,8,num"` // número remessa/retorno
	DataGravacao    time.Time `cnab:"192,8,date"`
	DataCredito     time.Time `cnab:"200,8,date"`
	CNAB3           string    `cnab:"208,33,alfa"`
}

// Operação codes of the batch header.
const (
	OperacaoRemessa = "R"
	OperacaoRetorno = "T"
)

// ServicoCobranca is the batch header service type for boleto collection.
const ServicoCobranca = 1

// BatchTrailer is the cobrança batch trailer, record type 5 ("trailer de
// lote"). QuantidadeRegistros counts every record of the batch including
// its header and trailer. The per-carteira totals are normally filled by
// the bank in retorno files and zero in remessa files; banks differ on
// whether they refer to the file or to the whole portfolio.
type BatchTrailer struct {
	Controle
	Registro              string `cnab:"8,1,const=5"`
	CNAB1                 string `cnab:"9,9,alfa"`
	QuantidadeRegistros   int    `cnab:"18,6,num"`
	QuantidadeSimples     int    `cnab:"24,6,num"`
	ValorSimples          int64  `cnab:"30,17,money"`
	QuantidadeVinculada   int    `cnab:"47,6,num"`
	ValorVinculada        int64  `cnab:"53,17,money"`
	QuantidadeCaucionada  int    `cnab:"70,6,num"`
	ValorCaucionada       int64  `cnab:"76,17,money"`
	QuantidadeDescontada  int    `cnab:"93,6,num"`
	ValorDescontada       int64  `cnab:"99,17,money"`
	NumeroAvisoLancamento string `cnab:"116,8,num"`
	CNAB2                 string `cnab:"124,117,alfa"`
}

// FileTrailer is the file trailer, record type 9 ("trailer de arquivo").
// QuantidadeRegistros counts every line of the file, headers and trailers
// included.
type FileTrailer struct {
	Controle
	Registro            string `cnab:"8,1,const=9"`
	CNAB1               string `cnab:"9,9,alfa"`
	QuantidadeLotes     int    `cnab:"18,6,num"`
	QuantidadeRegistros int    `cnab:"24,6,num"`
	QuantidadeContas    int    `cnab:"30,6,num"` // contas para conciliação
	CNAB2               string `cnab:"36,205,alfa"`
}

// DetalheControle holds positions 1-13 of every detail segment: control
// fields, record type 3 and the record's sequence number within the batch.
type DetalheControle struct {
	Controle
	Registro   string `cnab:"8,1,const=3"`
	Sequencial int    `cnab:"9,5,num"`
}
