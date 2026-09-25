package parser

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var (
	// ErrSemValor reports input with no recognizable number.
	ErrSemValor = errors.New("parser: no monetary amount in input")
	// ErrValorInvalido reports a number that is present but out of range.
	ErrValorInvalido = errors.New("parser: invalid monetary amount")
	// ErrValorNaoPositivo reports a zero amount, which describes no entry.
	ErrValorNaoPositivo = errors.New("parser: amount must be greater than zero")
	// ErrEntradaLonga reports input longer than MaxEntrada.
	ErrEntradaLonga = errors.New("parser: input too long")
)

// MaxEntrada caps the input size, in bytes.
//
// The parser receives free text from the user, and that is a trust boundary:
// without a ceiling, a megabyte-sized input becomes proportional CPU and memory
// work. Go's regexp is RE2 and immune to ReDoS, but linear over huge input is
// still huge. 200 bytes is plenty for any one-line entry.
const MaxEntrada = 200

// reValor matches a pt-BR monetary number: digits, thousands dots and an
// optional decimal part after the comma. It matches the whole token at once
// ("1.234,56") rather than pieces, so a malformed number is never split in two.
//
// Go's regexp is RE2: linear, no backtracking. Hostile input cannot turn into
// ReDoS here, and swapping in a backtracking library would lose that guarantee.
var reValor = regexp.MustCompile(`\d[\d.]*(?:,\d+)?`)

// extrairCentavos returns the input's amount, in cents.
//
// pt-BR rules: the comma is the decimal separator, the dot is the thousands
// separator. When there is more than one number, the last one wins -- "2 cafes
// 15" is R$ 15,00. It is a rule that never rejects and never asks, because
// asking costs the habit.
func extrairCentavos(entrada string) (int64, error) {
	numeros := reValor.FindAllString(entrada, -1)
	if len(numeros) == 0 {
		return 0, ErrSemValor
	}

	inteiro, decimais, _ := strings.Cut(numeros[len(numeros)-1], ",")

	// Dots are thousands separators and carry no information: 1.234 is 1234.
	// There is no guard for an empty integer part: reValor requires a leading
	// digit, so the part before the comma always keeps at least one digit after
	// the dots are removed. An unreachable guard lies about what can happen.
	inteiro = strings.ReplaceAll(inteiro, ".", "")

	// Cents have exactly two digits: "5" becomes "50", "555" becomes "55".
	switch {
	case len(decimais) == 0:
		decimais = "00"
	case len(decimais) == 1:
		decimais += "0"
	case len(decimais) > 2:
		decimais = decimais[:2]
	}

	// Building the cents string and converting once leaves overflow to
	// ParseInt, instead of a multiplication that overflows silently.
	centavos, err := strconv.ParseInt(inteiro+decimais, 10, 64)
	if err != nil {
		return 0, ErrValorInvalido
	}
	if centavos == 0 {
		return 0, ErrValorNaoPositivo
	}
	return centavos, nil
}
