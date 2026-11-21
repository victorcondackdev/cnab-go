// Package docid validates Brazilian CPF and CNPJ numbers (digits only).
package docid

// Kind is the document type, matching the CNAB "tipo de inscrição" codes.
type Kind int

// Document kinds.
const (
	Invalid Kind = 0
	CPF     Kind = 1
	CNPJ    Kind = 2
)

// Classify returns CPF or CNPJ when digits is a valid 11- or 14-digit
// number (check digits verified, repeated-digit sequences rejected), and
// Invalid otherwise.
func Classify(digits string) Kind {
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return Invalid
		}
	}
	switch len(digits) {
	case 11:
		if allSame(digits) || !checkCPF(digits) {
			return Invalid
		}
		return CPF
	case 14:
		if allSame(digits) || !checkCNPJ(digits) {
			return Invalid
		}
		return CNPJ
	}
	return Invalid
}

func allSame(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}

func dv(sum int) byte {
	r := sum % 11
	if r < 2 {
		return '0'
	}
	return byte('0' + 11 - r)
}

func checkCPF(d string) bool {
	for n := 9; n <= 10; n++ {
		sum := 0
		for i := 0; i < n; i++ {
			sum += int(d[i]-'0') * (n + 1 - i)
		}
		if dv(sum) != d[n] {
			return false
		}
	}
	return true
}

func checkCNPJ(d string) bool {
	for n := 12; n <= 13; n++ {
		sum, w := 0, n-7
		for i := 0; i < n; i++ {
			sum += int(d[i]-'0') * w
			w--
			if w < 2 {
				w = 9
			}
		}
		if dv(sum) != d[n] {
			return false
		}
	}
	return true
}
