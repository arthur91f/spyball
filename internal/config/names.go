package config

import (
	"strings"
	"unicode"
)

// foldings replaces accented Latin letters and ligatures by their plain letters.
var foldings = strings.NewReplacer(
	"à", "a", "á", "a", "â", "a", "ã", "a", "ä", "a", "å", "a", "æ", "ae",
	"ç", "c",
	"è", "e", "é", "e", "ê", "e", "ë", "e",
	"ì", "i", "í", "i", "î", "i", "ï", "i",
	"ñ", "n",
	"ò", "o", "ó", "o", "ô", "o", "õ", "o", "ö", "o", "ø", "o", "œ", "oe",
	"ù", "u", "ú", "u", "û", "u", "ü", "u",
	"ý", "y", "ÿ", "y",
	"ß", "ss",
)

// NormalizeName is the form under which character and player names are compared, so that they are easy
// to type on a phone: case, accents, spaces and punctuation are ignored. "ONG P.L.S." and "ong pls" match,
// so do "Rémi" and "remi", or "Corée du Nord" and "coree-du-nord".
func NormalizeName(name string) string {
	folded := foldings.Replace(strings.ToLower(name))
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, folded)
}
