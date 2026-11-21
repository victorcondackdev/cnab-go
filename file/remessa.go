package file

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/victorcondackdev/cnab-go/cnab240"
	"github.com/victorcondackdev/cnab-go/fixedwidth"
	"github.com/victorcondackdev/cnab-go/internal/docid"
)

// ErrInvalidBoleto reports builder input that cannot produce a valid
// remessa. The concrete errors are joined with [errors.Join].
var ErrInvalidBoleto = errors.New("cnab: invalid remessa data")

// Company identifies the beneficiary ("beneficiário/cedente") as registered
// with the bank.
type Company struct {
	Nome           string
	Documento      string // CPF or CNPJ, digits (punctuation is stripped)
	Convenio       string // código do convênio / beneficiário, bank-defined
	Agencia        string
	AgenciaDV      string
	Conta          string
	ContaDV        string
	AgenciaContaDV string
}

// Bank identifies the bank receiving the remessa.
type Bank struct {
	Codigo string // compensation code, e.g. "341"
	Nome   string
	// VersaoArquivo and VersaoLote are the layout versions written at
	// positions 164-166 of the file header and 14-16 of the batch header.
	// They are bank-specific; the defaults ("103" and "060") are common
	// FEBRABAN values but must be checked against the bank's manual.
	VersaoArquivo string
	VersaoLote    string
}

// Pagador is the payer of a boleto.
type Pagador struct {
	Nome      string
	Documento string // CPF or CNPJ
	Endereco  string
	Bairro    string
	CEP       string // 8 digits, "-" allowed
	Cidade    string
	UF        string
}

// Boleto is one title to register. Amounts are in cents.
//
// Text longer than its CNAB field (names, addresses) is truncated after
// accent folding; everything else is validated and rejected when invalid.
type Boleto struct {
	NossoNumero     string // bank's title identifier, formatted as your bank requires
	NumeroDocumento string // "seu número", up to 15 characters
	Carteira        int    // defaults to 1 (cobrança simples)
	Especie         cnab240.Especie
	Aceite          bool
	Emissao         time.Time // defaults to the remessa date
	Vencimento      time.Time
	Valor           int64

	JurosPorDia  int64     // late interest per day; zero means exempt
	Desconto     int64     // fixed discount until DescontoAte
	DescontoAte  time.Time // defaults to Vencimento when Desconto > 0
	Abatimento   int64
	Multa        int64 // fixed late fine charged from the day after Vencimento (segment R)
	DiasProtesto int   // zero means "não protestar"
	DiasBaixa    int   // zero leaves the bank default
	UsoEmpresa   string
	Mensagem     string // printed message (segment R, mensagem 3)

	Pagador Pagador
}

// Remessa builds a CNAB 240 cobrança remessa with a single batch. Errors in
// AddBoleto are collected and reported by Build.
type Remessa struct {
	company Company
	bank    Bank
	nsa     int
	at      time.Time
	titles  []cnab240.Titulo
	errs    []error
}

// NewRemessa starts a remessa for company at bank. The file sequence number
// (NSA) defaults to 1 and the generation time to [time.Now].
func NewRemessa(company Company, bank Bank) *Remessa {
	return &Remessa{company: company, bank: bank, nsa: 1}
}

// WithNSA sets the file sequence number ("número sequencial do arquivo");
// banks require it to increase with every file sent.
func (r *Remessa) WithNSA(n int) *Remessa { r.nsa = n; return r }

// WithGeneratedAt sets the generation timestamp written in the headers.
func (r *Remessa) WithGeneratedAt(t time.Time) *Remessa { r.at = t; return r }

func digits(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		if r == '.' || r == '-' || r == '/' || r == ' ' {
			return -1
		}
		return 'x' // keep invalid input invalid
	}, s)
}

// text folds s to CNAB alfa (upper-case ASCII) and truncates it to n bytes.
func text(s string, n int) (string, error) {
	f, err := fixedwidth.Fold(strings.TrimSpace(s))
	if err != nil {
		return "", err
	}
	if len(f) > n {
		f = strings.TrimRight(f[:n], " ")
	}
	return f, nil
}

// AddBoleto validates b and appends its P, Q and (when needed) R segments.
func (r *Remessa) AddBoleto(b Boleto) *Remessa {
	idx := len(r.titles) + len(r.errs) + 1
	fail := func(format string, args ...any) *Remessa {
		r.errs = append(r.errs, fmt.Errorf("boleto %d (nosso número %q): "+format, append([]any{idx, b.NossoNumero}, args...)...))
		return r
	}
	nn, err := text(b.NossoNumero, 20)
	switch {
	case err != nil || nn == "":
		return fail("nosso número is required ASCII text")
	case len(strings.TrimSpace(b.NossoNumero)) > 20:
		return fail("nosso número longer than 20 characters")
	case b.Valor <= 0:
		return fail("valor must be positive")
	case b.Vencimento.IsZero():
		return fail("vencimento is required")
	case b.NumeroDocumento == "" || len(b.NumeroDocumento) > 15:
		return fail("número do documento must have 1 to 15 characters")
	case b.JurosPorDia < 0 || b.Desconto < 0 || b.Abatimento < 0 || b.Multa < 0:
		return fail("amounts must not be negative")
	case b.DiasProtesto < 0 || b.DiasProtesto > 99 || b.DiasBaixa < 0 || b.DiasBaixa > 999:
		return fail("dias de protesto/baixa out of range")
	}
	doc := digits(b.Pagador.Documento)
	kind := docid.Classify(doc)
	if kind == docid.Invalid {
		return fail("pagador documento %q is not a valid CPF or CNPJ", b.Pagador.Documento)
	}
	cep := digits(b.Pagador.CEP)
	if len(cep) != 8 || strings.Contains(cep, "x") {
		return fail("pagador CEP %q must have 8 digits", b.Pagador.CEP)
	}
	numDoc, err := text(b.NumeroDocumento, 15)
	if err != nil {
		return fail("número do documento: %v", err)
	}
	var q cnab240.SegmentQ
	fields := []struct {
		dst *string
		src string
		n   int
	}{
		{&q.Nome, b.Pagador.Nome, 40}, {&q.Endereco, b.Pagador.Endereco, 40},
		{&q.Bairro, b.Pagador.Bairro, 15}, {&q.Cidade, b.Pagador.Cidade, 15}, {&q.UF, b.Pagador.UF, 2},
	}
	for _, f := range fields {
		if *f.dst, err = text(f.src, f.n); err != nil {
			return fail("pagador: %v", err)
		}
	}
	if q.Nome == "" || len(q.UF) != 2 {
		return fail("pagador nome and a 2-letter UF are required")
	}
	q.Movimento = cnab240.RemessaEntrada
	q.TipoInscricao = int(kind)
	q.NumeroInscricao = doc
	q.CEP, q.SufixoCEP = cep[:5], cep[5:]

	c := r.company
	p := &cnab240.SegmentP{
		Movimento:          cnab240.RemessaEntrada,
		Agencia:            c.Agencia,
		AgenciaDV:          c.AgenciaDV,
		Conta:              c.Conta,
		ContaDV:            c.ContaDV,
		AgenciaContaDV:     c.AgenciaContaDV,
		NossoNumero:        nn,
		Carteira:           b.Carteira,
		FormaCadastramento: 1,   // com cadastramento (registrada)
		TipoDocumento:      "1", // tradicional
		EmissaoBoleto:      2,   // cliente emite
		DistribuicaoBoleto: "2", // cliente distribui
		NumeroDocumento:    numDoc,
		Vencimento:         b.Vencimento,
		Valor:              b.Valor,
		Especie:            b.Especie,
		Aceite:             "N",
		DataEmissao:        b.Emissao,
		CodigoJuros:        cnab240.JurosIsento,
		Abatimento:         b.Abatimento,
		CodigoProtesto:     cnab240.NaoProtestar,
		Moeda:              cnab240.MoedaReal,
	}
	if p.Carteira == 0 {
		p.Carteira = 1
	}
	if p.Especie == 0 {
		p.Especie = cnab240.EspecieDuplicataMercantil
	}
	if b.Aceite {
		p.Aceite = "A"
	}
	if b.JurosPorDia > 0 {
		p.CodigoJuros, p.DataJuros, p.Juros = cnab240.JurosValorDiario, b.Vencimento.AddDate(0, 0, 1), b.JurosPorDia
	}
	if b.Desconto > 0 {
		p.CodigoDesconto, p.Desconto, p.DataDesconto = cnab240.DescontoValorFixo, b.Desconto, b.DescontoAte
		if p.DataDesconto.IsZero() {
			p.DataDesconto = b.Vencimento
		}
	}
	if b.DiasProtesto > 0 {
		p.CodigoProtesto, p.DiasProtesto = cnab240.ProtestarDiasCorridos, b.DiasProtesto
	}
	if b.DiasBaixa > 0 {
		p.CodigoBaixa, p.DiasBaixa = cnab240.BaixarDevolver, fmt.Sprintf("%03d", b.DiasBaixa)
	}
	if p.UsoEmpresa, err = text(b.UsoEmpresa, 25); err != nil {
		return fail("uso da empresa: %v", err)
	}
	t := cnab240.Titulo{P: p, Q: &q}
	if b.Multa > 0 || b.Mensagem != "" {
		rr := &cnab240.SegmentR{Movimento: cnab240.RemessaEntrada}
		if b.Multa > 0 {
			rr.CodigoMulta, rr.DataMulta, rr.Multa = cnab240.MultaValorFixo, b.Vencimento.AddDate(0, 0, 1), b.Multa
		}
		if rr.Mensagem3, err = text(b.Mensagem, 40); err != nil {
			return fail("mensagem: %v", err)
		}
		t.R = rr
	}
	r.titles = append(r.titles, t)
	return r
}

// Build validates the company and bank data and returns the remessa as a
// [cnab240.File]. Control fields (batch numbers, sequences, counts) are
// filled in by [Write]; Build already sets them so the result is also
// consistent in memory.
func (r *Remessa) Build() (*cnab240.File, error) {
	errs := append([]error(nil), r.errs...)
	c, bk := r.company, r.bank
	doc := digits(c.Documento)
	kind := docid.Classify(doc)
	if kind == docid.Invalid {
		errs = append(errs, fmt.Errorf("company documento %q is not a valid CPF or CNPJ", c.Documento))
	}
	if len(bk.Codigo) != 3 || strings.Trim(bk.Codigo, "0123456789") != "" {
		errs = append(errs, fmt.Errorf("bank code %q must have 3 digits", bk.Codigo))
	}
	nome, err := text(c.Nome, 30)
	if err != nil || nome == "" {
		errs = append(errs, fmt.Errorf("company nome is required ASCII text"))
	}
	nomeBanco, err := text(bk.Nome, 30)
	if err != nil {
		errs = append(errs, fmt.Errorf("bank nome: %v", err))
	}
	if len(r.titles) == 0 && len(r.errs) == 0 {
		errs = append(errs, errors.New("no boletos added"))
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("%w: %w", ErrInvalidBoleto, errors.Join(errs...))
	}
	at := r.at
	if at.IsZero() {
		at = time.Now()
	}
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	va, vl := bk.VersaoArquivo, bk.VersaoLote
	if va == "" {
		va = "103"
	}
	if vl == "" {
		vl = "060"
	}
	ctl := cnab240.Controle{Banco: bk.Codigo}
	f := &cnab240.File{
		Header: cnab240.FileHeader{
			Controle:             ctl,
			TipoInscricao:        int(kind),
			NumeroInscricao:      doc,
			Convenio:             c.Convenio,
			Agencia:              c.Agencia,
			AgenciaDV:            c.AgenciaDV,
			Conta:                c.Conta,
			ContaDV:              c.ContaDV,
			AgenciaContaDV:       c.AgenciaContaDV,
			NomeEmpresa:          nome,
			NomeBanco:            nomeBanco,
			CodigoRemessaRetorno: cnab240.CodigoRemessa,
			DataGeracao:          day,
			HoraGeracao:          at.Format("150405"),
			NSA:                  r.nsa,
			VersaoLayout:         va,
		},
	}
	b := &cnab240.Batch{Header: cnab240.BatchHeader{
		Controle:        cnab240.Controle{Banco: bk.Codigo, Lote: 1},
		Operacao:        cnab240.OperacaoRemessa,
		Servico:         cnab240.ServicoCobranca,
		VersaoLayout:    vl,
		TipoInscricao:   int(kind),
		NumeroInscricao: doc,
		Convenio:        c.Convenio,
		Agencia:         c.Agencia,
		AgenciaDV:       c.AgenciaDV,
		Conta:           c.Conta,
		ContaDV:         c.ContaDV,
		AgenciaContaDV:  c.AgenciaContaDV,
		NomeEmpresa:     nome,
		NumeroRemessa:   r.nsa,
		DataGravacao:    day,
	}}
	seq := 0
	add := func(s cnab240.Segment, dc *cnab240.DetalheControle) {
		seq++
		dc.Controle = cnab240.Controle{Banco: bk.Codigo, Lote: 1}
		dc.Sequencial = seq
		b.Segments = append(b.Segments, s)
	}
	for _, t := range r.titles {
		p, q := *t.P, *t.Q
		if p.DataEmissao.IsZero() {
			p.DataEmissao = day
		}
		add(&p, &p.DetalheControle)
		add(&q, &q.DetalheControle)
		if t.R != nil {
			rr := *t.R
			add(&rr, &rr.DetalheControle)
		}
	}
	b.Trailer = cnab240.BatchTrailer{Controle: cnab240.Controle{Banco: bk.Codigo, Lote: 1}, QuantidadeRegistros: seq + 2}
	f.Batches = []*cnab240.Batch{b}
	f.Trailer = cnab240.FileTrailer{
		Controle:            cnab240.Controle{Banco: bk.Codigo, Lote: 9999},
		QuantidadeLotes:     1,
		QuantidadeRegistros: seq + 4,
	}
	// Encode every record once so field errors (for example non-digit
	// agência) surface here rather than in Write.
	if err := Write(io.Discard, f); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidBoleto, err)
	}
	return f, nil
}

// WriteTo builds the remessa and writes it with the default [Writer]. It
// implements [io.WriterTo].
func (r *Remessa) WriteTo(w io.Writer) (int64, error) {
	f, err := r.Build()
	if err != nil {
		return 0, err
	}
	cw := &countingWriter{w: w}
	err = Write(cw, f)
	return cw.n, err
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}
