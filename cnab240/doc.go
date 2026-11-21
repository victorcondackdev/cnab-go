// Package cnab240 declares the FEBRABAN CNAB 240 records used for boleto
// collection ("cobrança"): remessa files sent by the company to the bank and
// retorno files sent back by the bank.
//
// Every record is a plain struct whose cnab tags give its positions, so the
// layouts double as documentation and are self-validated by
// [github.com/victorcondackdev/cnab-go/fixedwidth]. Field names follow the
// Portuguese names used in the FEBRABAN manual, which is what bank analysts
// and integration manuals use.
//
// A CNAB 240 file is a sequence of 240-byte records:
//
//	0  file header        (lote 0000)
//	1  batch header       (lote 0001..)
//	3  detail segments    P, Q, R (remessa) / T, U (retorno), others kept raw
//	5  batch trailer
//	9  file trailer       (lote 9999)
//
// Positions follow the generic FEBRABAN "Layout Padrão CNAB 240" for
// cobrança (versions 08.x to 10.x share these positions). Banks publish
// their own manuals that fill the bank-reserved areas, restrict codes and
// sometimes deviate; the fields most often redefined are noted in the
// struct documentation. Always validate a new integration against your
// bank's homologation environment.
//
// Parsing and writing whole files (record counts, sequence numbers, batch
// grouping) lives in package [github.com/victorcondackdev/cnab-go/file].
package cnab240
