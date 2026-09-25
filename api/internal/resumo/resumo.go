// Package resumo aggregates entries into the numbers the dashboard shows.
//
// Pure domain: no I/O, no database, no network.
//
// The rule that justifies this package is not the sum, it is the difference
// between two numbers that look the same: a cash/debit expense weighs on the
// day it happened, a credit card expense weighs on the statement it lands on.
// An installment purchase made today shows up in several months; a debit
// purchase made today shows up only in this one. Summing both by purchase date
// is the easiest way to show a wrong number that looks right.
package resumo

import (
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/fatura"
	"github.com/augustodbatista/finance-platform/api/internal/parser"
)

// Resumo holds the numbers for one month.
//
// ponytail: no "current balance" yet. A balance needs an opening balance and
// statement payments modeled as entries -- neither exists. It arrives with the
// slice that introduces accounts and statement payments.
type Resumo struct {
	ReceitasCentavos int64
	DespesasCentavos int64
	// EconomiaCentavos is income minus expenses. It goes negative in a month
	// that only has a statement to pay, and that is information, not an error.
	EconomiaCentavos int64
}

// Mensal sums the entries that belong to the given month.
//
// Income counts in the month of its date and ignores statements and
// installments: a credit card is a way of spending, not of receiving.
//
// A credit card expense counts on each installment's statement; any other
// payment method counts in the month of the purchase date. FormaNaoInformada
// (method not stated) counts in the month of the date -- the parser does not
// invent a payment method, and treating the unknown as credit would postpone
// money that may already have left the account.
func Mensal(mes fatura.Competencia, lancamentos []parser.Lancamento, diaFechamento int) (Resumo, error) {
	var r Resumo

	for _, l := range lancamentos {
		if l.Tipo == parser.Receita {
			if noMes(l.Data, mes) {
				r.ReceitasCentavos += l.Centavos
			}
			continue
		}

		if l.Forma != parser.Credito {
			if noMes(l.Data, mes) {
				r.DespesasCentavos += l.Centavos
			}
			continue
		}

		parcelas, err := fatura.Dividir(l.Centavos, l.Parcelas, l.Data, diaFechamento)
		if err != nil {
			return Resumo{}, err
		}
		for _, p := range parcelas {
			if p.Competencia == mes {
				r.DespesasCentavos += p.Centavos
			}
		}
	}

	r.EconomiaCentavos = r.ReceitasCentavos - r.DespesasCentavos
	return r, nil
}

// noMes reports whether the date falls in the given month.
func noMes(data time.Time, mes fatura.Competencia) bool {
	return data.Year() == mes.Ano && data.Month() == mes.Mes
}
