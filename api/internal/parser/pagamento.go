package parser

import (
	"strings"

	"github.com/augustodbatista/finance-platform/api/internal/dominio"
)

// formas maps a typed word to a payment method. Keys are already normalized.
var formas = map[string]dominio.FormaPagamento{
	"dinheiro": dominio.Dinheiro,
	"especie":  dominio.Dinheiro,
	"cash":     dominio.Dinheiro,

	"pix": dominio.Pix,

	"debito": dominio.Debito,

	"credito": dominio.Credito,
	// "cartao" (card) alone is ambiguous. Credit is the majority reading, and the
	// same logic as Outros -> Despesa applies: picking the most likely case
	// costs less than asking. People paying by debit usually say "debito".
	"cartao": dominio.Credito,
}

// formaDe returns the payment method mentioned in the input, or
// dominio.FormaNaoInformada. Expects normalized input (see normalizar).
func formaDe(entrada string) dominio.FormaPagamento {
	for _, palavra := range strings.Fields(entrada) {
		if f, ok := formas[palavra]; ok {
			return f
		}
	}
	return dominio.FormaNaoInformada
}
