package fixedwidth

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// foldTable maps Latin-1 letters and a few common symbols to their ASCII
// equivalents. Upper-casing happens after the lookup.
var foldTable = map[rune]string{
	'á': "A", 'à': "A", 'â': "A", 'ã': "A", 'ä': "A", 'å': "A",
	'Á': "A", 'À': "A", 'Â': "A", 'Ã': "A", 'Ä': "A", 'Å': "A",
	'é': "E", 'è': "E", 'ê': "E", 'ë': "E", 'É': "E", 'È': "E", 'Ê': "E", 'Ë': "E",
	'í': "I", 'ì': "I", 'î': "I", 'ï': "I", 'Í': "I", 'Ì': "I", 'Î': "I", 'Ï': "I",
	'ó': "O", 'ò': "O", 'ô': "O", 'õ': "O", 'ö': "O", 'Ó': "O", 'Ò': "O", 'Ô': "O", 'Õ': "O", 'Ö': "O",
	'ú': "U", 'ù': "U", 'û': "U", 'ü': "U", 'Ú': "U", 'Ù': "U", 'Û': "U", 'Ü': "U",
	'ç': "C", 'Ç': "C", 'ñ': "N", 'Ñ': "N", 'ý': "Y", 'ÿ': "Y", 'Ý': "Y",
	'º': "O", 'ª': "A", '°': "O",
}

// Fold returns s upper-cased with accents folded to ASCII, exactly as an
// alfa field is encoded. It fails on control characters and on runes with
// no ASCII equivalent. Folding never changes the byte length of ASCII input
// and never makes a string longer than its rune count.
func Fold(s string) (string, error) { return fold(s) }

func fold(s string) (string, error) {
	ascii := true
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x20 || c > 0x7e || (c >= 'a' && c <= 'z') {
			ascii = false
			break
		}
	}
	if ascii {
		return s, nil
	}
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range s {
		switch {
		case r == utf8.RuneError && !strings.HasPrefix(s[i:], "�"):
			return "", errors.New("invalid UTF-8")
		case r >= 'a' && r <= 'z':
			b.WriteByte(byte(r) - 'a' + 'A')
		case r >= 0x20 && r <= 0x7e:
			b.WriteByte(byte(r))
		default:
			rep, ok := foldTable[r]
			if !ok {
				return "", fmt.Errorf("rune %q has no ASCII equivalent", r)
			}
			b.WriteString(rep)
		}
	}
	return b.String(), nil
}
