// Package resumo aggregates entries into the numbers the dashboard shows.
//
// Pure domain: no I/O, no database, no network.
//
// The rule that justifies this package is not the sum, it is the difference
// between numbers that look the same: a cash/debit expense leaves the pocket on
// the day it happened; a credit card purchase lands on a statement, and only
// leaves the pocket when that statement is paid. Summing everything by purchase
// date is the easiest way to show a wrong number that looks right.
package resumo

import (
	"cmp"
	"slices"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/dominio"
	"github.com/augustodbatista/finance-platform/api/internal/fatura"
)

// Resumo holds the numbers for one month.
//
// ponytail: no "current balance" yet. A balance needs an opening balance per
// account, which does not exist. It arrives with the slice that brings accounts.
type Resumo struct {
	ReceitasCentavos int64
	DespesasCentavos int64
	// EconomiaCentavos is income minus expenses. It goes negative in a month
	// that only has a statement to pay, and that is information, not an error.
	EconomiaCentavos int64
	// Categorias breaks DespesasCentavos down by category, largest first. It
	// always adds up to DespesasCentavos: both are filled at the same point.
	Categorias []TotalCategoria
	// APagarCentavos is what is due this month and not paid yet: bills by their
	// expected amount, plus this month's credit card statement if unpaid. It is
	// not part of DespesasCentavos, which only counts money that already left
	// the pocket.
	APagarCentavos int64
}

// TotalCategoria is how much of the month's expenses went to one category.
type TotalCategoria struct {
	Categoria dominio.Categoria
	Centavos  int64
}

// Mensal sums the entries that belong to the given month.
//
// Income counts in the month of its date and ignores statements and
// installments: a credit card is a way of spending, not of receiving.
//
// A bill counts as an expense only once paid, in the month of the payment and
// for the amount paid; until then it is "a pagar" in the month it is due.
//
// Credit card statements follow the same rule. Each installment lands on a
// statement; an unpaid statement is "a pagar" in its own month; a paid one is
// an expense in the month of the payment (pagas), split by the categories of
// its installments. When the amount paid differs from the installments' total
// (interest, purchases never logged, a refund), the difference goes to
// dominio.AjusteFatura, so the categories still add up to the expenses.
//
// Any other payment method counts in the month of the purchase date.
// FormaNaoInformada (method not stated) counts there too -- the parser does not
// invent a payment method, and treating the unknown as credit would postpone
// money that may already have left the account.
func Mensal(mes fatura.Competencia, lancamentos []dominio.Lancamento, pagas []fatura.Pagamento, diaFechamento int) (Resumo, error) {
	faturas, err := porFatura(lancamentos, diaFechamento)
	if err != nil {
		return Resumo{}, err
	}

	var r Resumo
	porCategoria := map[dominio.Categoria]int64{}

	// The only place an expense is counted: the total and the per-category
	// breakdown cannot drift apart.
	despesa := func(c dominio.Categoria, centavos int64) {
		r.DespesasCentavos += centavos
		porCategoria[c] += centavos
	}

	for _, l := range lancamentos {
		if l.Tipo == dominio.Receita {
			if noMes(l.Data, mes) {
				r.ReceitasCentavos += l.Centavos
			}
			continue
		}

		if l.EConta() {
			switch {
			case l.Pago() && noMes(l.Pagamento.Data, mes):
				despesa(l.Categoria, l.Pagamento.Centavos)
			case !l.Pago() && noMes(l.Vencimento, mes):
				r.APagarCentavos += l.Centavos
			}
			continue
		}

		if l.Forma != dominio.Credito && noMes(l.Data, mes) {
			despesa(l.Categoria, l.Centavos)
		}
	}

	paga := map[fatura.Competencia]bool{}
	for _, p := range pagas {
		paga[p.Competencia] = true
		if !noMes(p.Data, mes) {
			continue
		}
		var total int64
		for c, v := range faturas[p.Competencia] {
			despesa(c, v)
			total += v
		}
		if ajuste := p.Centavos - total; ajuste != 0 {
			despesa(dominio.AjusteFatura, ajuste)
		}
	}
	if !paga[mes] {
		r.APagarCentavos += soma(faturas[mes])
	}

	r.EconomiaCentavos = r.ReceitasCentavos - r.DespesasCentavos
	r.Categorias = ordenar(porCategoria)
	return r, nil
}

// Fatura is one credit card statement: what its installments add up to, and
// its payment, if any.
type Fatura struct {
	Competencia fatura.Competencia
	Centavos    int64
	Pagamento   *fatura.Pagamento // nil while unpaid
}

// Faturas lists every statement that has installments or a payment, newest
// first. A payment whose installments were all deleted is still listed (with
// Centavos 0): it still counts as an expense, so it must stay visible and
// undoable.
//
// ponytail: lists every statement ever, including far-future installments.
// Paginate or cut by date if the list gets long on screen.
func Faturas(lancamentos []dominio.Lancamento, pagas []fatura.Pagamento, diaFechamento int) ([]Fatura, error) {
	faturas, err := porFatura(lancamentos, diaFechamento)
	if err != nil {
		return nil, err
	}
	porComp := map[fatura.Competencia]*Fatura{}
	for c, cats := range faturas {
		porComp[c] = &Fatura{Competencia: c, Centavos: soma(cats)}
	}
	for _, p := range pagas {
		f := porComp[p.Competencia]
		if f == nil {
			f = &Fatura{Competencia: p.Competencia}
			porComp[p.Competencia] = f
		}
		f.Pagamento = &p
	}

	out := make([]Fatura, 0, len(porComp))
	for _, f := range porComp {
		out = append(out, *f)
	}
	slices.SortFunc(out, func(a, b Fatura) int {
		return cmp.Or(cmp.Compare(b.Competencia.Ano, a.Competencia.Ano), cmp.Compare(b.Competencia.Mes, a.Competencia.Mes))
	})
	return out, nil
}

// porFatura splits every credit expense into installments and adds them up by
// statement and category.
func porFatura(lancamentos []dominio.Lancamento, diaFechamento int) (map[fatura.Competencia]map[dominio.Categoria]int64, error) {
	out := map[fatura.Competencia]map[dominio.Categoria]int64{}
	for _, l := range lancamentos {
		if l.Tipo == dominio.Receita || l.Forma != dominio.Credito {
			continue
		}
		parcelas, err := fatura.Dividir(l.Centavos, l.Parcelas, l.Data, diaFechamento)
		if err != nil {
			return nil, err
		}
		for _, p := range parcelas {
			if out[p.Competencia] == nil {
				out[p.Competencia] = map[dominio.Categoria]int64{}
			}
			out[p.Competencia][l.Categoria] += p.Centavos
		}
	}
	return out, nil
}

func soma(porCategoria map[dominio.Categoria]int64) int64 {
	var total int64
	for _, v := range porCategoria {
		total += v
	}
	return total
}

// ordenar lists categories largest first, ties by name, so the screen never
// reshuffles between two loads of the same month (map order is random).
func ordenar(porCategoria map[dominio.Categoria]int64) []TotalCategoria {
	out := make([]TotalCategoria, 0, len(porCategoria))
	for c, v := range porCategoria {
		out = append(out, TotalCategoria{Categoria: c, Centavos: v})
	}
	slices.SortFunc(out, func(a, b TotalCategoria) int {
		return cmp.Or(cmp.Compare(b.Centavos, a.Centavos), cmp.Compare(a.Categoria, b.Categoria))
	})
	return out
}

// Vencidas counts the bills that are past due and still unpaid (see
// dominio.Lancamento.Vencida), whatever month is on screen: an overdue bill
// must not vanish just because the summary moved on to the next month.
func Vencidas(lancamentos []dominio.Lancamento, hoje time.Time) (quantidade int, centavos int64) {
	for _, l := range lancamentos {
		if l.Vencida(hoje) {
			quantidade++
			centavos += l.Centavos
		}
	}
	return quantidade, centavos
}

// noMes reports whether the date falls in the given month.
func noMes(data time.Time, mes fatura.Competencia) bool {
	return data.Year() == mes.Ano && data.Month() == mes.Mes
}
