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
	"cmp"
	"slices"
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
	// Categorias breaks DespesasCentavos down by category, largest first. It
	// always adds up to DespesasCentavos: both are filled at the same point.
	Categorias []TotalCategoria
}

// TotalCategoria is how much of the month's expenses went to one category.
type TotalCategoria struct {
	Categoria parser.Categoria
	Centavos  int64
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
	porCategoria := map[parser.Categoria]int64{}

	// The only place an expense is counted: the total and the per-category
	// breakdown cannot drift apart.
	despesa := func(c parser.Categoria, centavos int64) {
		r.DespesasCentavos += centavos
		porCategoria[c] += centavos
	}

	for _, l := range lancamentos {
		if l.Tipo == parser.Receita {
			if noMes(l.Data, mes) {
				r.ReceitasCentavos += l.Centavos
			}
			continue
		}

		if l.Forma != parser.Credito {
			if noMes(l.Data, mes) {
				despesa(l.Categoria, l.Centavos)
			}
			continue
		}

		parcelas, err := fatura.Dividir(l.Centavos, l.Parcelas, l.Data, diaFechamento)
		if err != nil {
			return Resumo{}, err
		}
		for _, p := range parcelas {
			if p.Competencia == mes {
				despesa(l.Categoria, p.Centavos)
			}
		}
	}

	r.EconomiaCentavos = r.ReceitasCentavos - r.DespesasCentavos
	r.Categorias = ordenar(porCategoria)
	return r, nil
}

// ordenar lists categories largest first, ties by name, so the screen never
// reshuffles between two loads of the same month (map order is random).
func ordenar(porCategoria map[parser.Categoria]int64) []TotalCategoria {
	out := make([]TotalCategoria, 0, len(porCategoria))
	for c, v := range porCategoria {
		out = append(out, TotalCategoria{Categoria: c, Centavos: v})
	}
	slices.SortFunc(out, func(a, b TotalCategoria) int {
		return cmp.Or(cmp.Compare(b.Centavos, a.Centavos), cmp.Compare(a.Categoria, b.Categoria))
	})
	return out
}

// noMes reports whether the date falls in the given month.
func noMes(data time.Time, mes fatura.Competencia) bool {
	return data.Year() == mes.Ano && data.Month() == mes.Mes
}
