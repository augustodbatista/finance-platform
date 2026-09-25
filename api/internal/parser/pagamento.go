package parser

import "strings"

// FormaPagamento says how money left or came in.
//
// It matters more than it looks: a credit card purchase does not change the
// balance today, only when the statement is paid. Without this, the dashboard
// lies to anyone who uses a card, which is almost everyone.
type FormaPagamento string

const (
	// FormaNaoInformada is the zero value: the user did not say.
	//
	// The parser reports what it found and does not invent a default. Applying
	// the user's preference is the job of the layer that knows user settings --
	// the pure domain should not know people's preferences.
	FormaNaoInformada FormaPagamento = ""

	Dinheiro FormaPagamento = "dinheiro"
	Pix      FormaPagamento = "pix"
	Debito   FormaPagamento = "debito"
	Credito  FormaPagamento = "credito"
)

// formas maps a typed word to a payment method. Keys are already normalized.
var formas = map[string]FormaPagamento{
	"dinheiro": Dinheiro,
	"especie":  Dinheiro,
	"cash":     Dinheiro,

	"pix": Pix,

	"debito": Debito,

	"credito": Credito,
	// "cartao" (card) alone is ambiguous. Credit is the majority reading, and the
	// same logic as Outros -> Despesa applies: picking the most likely case
	// costs less than asking. People paying by debit usually say "debito".
	"cartao": Credito,
}

// formaDe returns the payment method mentioned in the input, or
// FormaNaoInformada. Expects normalized input (see normalizar).
func formaDe(entrada string) FormaPagamento {
	for _, palavra := range strings.Fields(entrada) {
		if f, ok := formas[palavra]; ok {
			return f
		}
	}
	return FormaNaoInformada
}
