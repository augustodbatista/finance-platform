package parser

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/augustodbatista/finance-platform/api/internal/fatura"
)

// reParcelas matches "3x", "3 x" and "12x1200".
//
// No trailing \b on purpose: with it, "12x1200" would not match, because there
// is no word boundary between the "x" and the "1" -- and the token would stay
// in the string for the amount extractor to misread.
var reParcelas = regexp.MustCompile(`\b(\d+)\s*x`)

// extrairParcelas returns the number of installments and the input without
// the token.
//
// No token means one installment, not zero: that way summing installments
// works the same for single and installment purchases, with no special case.
//
// Returning the cleaned input is correctness, not convenience. The "last
// number wins" rule would turn "300 mercado 3x" into R$ 3,00 if the token were
// still in the string when the amount is extracted.
//
// The ceiling and the error come from package fatura, not from here: how many
// installments a card accepts is a card rule, not a text rule. Duplicating the
// limit in both packages would be two truths about the same thing, ready to
// drift apart.
//
// Expects normalized input (see normalizar).
func extrairParcelas(entrada string) (int, string, error) {
	m := reParcelas.FindStringSubmatch(entrada)
	if m == nil {
		return 1, entrada, nil
	}

	// Atoi also fails on overflow, not only on invalid text: a 23-digit number
	// ends up here instead of becoming silent garbage.
	parcelas, err := strconv.Atoi(m[1])
	if err != nil || parcelas < 1 || parcelas > fatura.MaxParcelas {
		return 0, "", fatura.ErrParcelasInvalidas
	}

	return parcelas, strings.Replace(entrada, m[0], " ", 1), nil
}
