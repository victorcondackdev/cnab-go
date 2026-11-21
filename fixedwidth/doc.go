// Package fixedwidth is a declarative, struct-tag-driven codec for
// fixed-width records such as the FEBRABAN CNAB 240 and CNAB 400 layouts.
//
// A record is described by a struct whose fields carry a cnab tag giving the
// field's position exactly as printed in bank manuals (1-based, inclusive)
// and its format:
//
//	type Header struct {
//		Banco    string    `cnab:"1,3,num"`       // start 1, length 3
//		Lote     int       `cnab:"4-7,num"`       // start 4, end 7
//		Registro string    `cnab:"8,1,const=0"`   // fixed literal
//		Nome     string    `cnab:"9,30,alfa"`
//		Data     time.Time `cnab:"39,8,date"`     // DDMMYYYY
//		Valor    int64     `cnab:"47,15,money"`   // cents, 2 implied decimals
//		Livre    string    `cnab:"62,179,raw"`
//	}
//
// The position is either "start,length" or "start-end". Formats:
//
//   - num: digits, right-aligned and zero-padded. Go type string (kept
//     verbatim, so leading zeros of agência or CNPJ survive) or any integer
//     kind (width at most 18 digits). An all-blank field decodes as "" or 0;
//     many banks leave unused numeric fields blank.
//   - alfa: left-aligned, space-padded. On encode the value is upper-cased
//     and accents are folded to ASCII ("São João" → "SAO JOAO"); any other
//     non-printable or non-ASCII rune is an error. On decode trailing spaces
//     are trimmed. Values longer than the field are an error, never silently
//     truncated.
//   - raw: like alfa but without case folding or trimming. Use it for
//     bank-reserved areas so round trips are lossless.
//   - date: time.Time. Width 8 is DDMMYYYY, width 6 is DDMMYY (two-digit
//     years 69–99 map to 19xx, 00–68 to 20xx, as in [time.Parse]). All zeros
//     or all blanks mean "no date" and decode to the zero time.
//   - money: an integer kind holding cents. The field carries two implied
//     decimal places, so "000000000012345" is 12345 cents (R$ 123,45).
//     Negative values are rejected: CNAB has no sign.
//   - const=LITERAL: a string field whose content must equal LITERAL (same
//     length as the field). Encode writes the literal when the field is
//     empty; decode fails if the line holds anything else.
//
// Fields without a cnab tag, or tagged "-", are ignored. Anonymous embedded
// structs are flattened, so shared prefixes (bank code, batch number) can be
// declared once.
//
// # Layout self-validation
//
// The first time a type is used its layout is compiled and checked: every
// position from 1 to the last one must be covered by exactly one field. Gaps,
// overlaps, malformed tags, format/type mismatches and integer fields too
// wide for their Go type are reported together in a [*LayoutError] that names
// the offending fields. A typo copied from a manual ("86,15" written as
// "87,15") therefore fails the first unit test that touches the type instead
// of corrupting files in production. [Check] runs the same validation
// explicitly. Compiled layouts are cached per type.
//
// # Adding a layout
//
// Write one struct per record type, with one field per entry in the bank's
// manual, including filler ("Uso exclusivo FEBRABAN/CNAB") fields, and add a
// test calling [Check] with the record length. That is all a new bank
// layout needs.
package fixedwidth
