package resumo_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/fatura"
	"github.com/augustodbatista/finance-platform/api/internal/parser"
	"github.com/augustodbatista/finance-platform/api/internal/resumo"
)

const fechamento = 28

func dia(ano int, mes time.Month, d int) time.Time {
	return time.Date(ano, mes, d, 0, 0, 0, 0, time.UTC)
}

func comp(ano int, mes time.Month) fatura.Competencia {
	return fatura.Competencia{Ano: ano, Mes: mes}
}

// lancamentos builds the same set for every test: a month with income, a debit
// expense, a credit card installment purchase, an entry from the previous month
// and a purchase made on the closing day itself.
func lancamentos() []parser.Lancamento {
	return []parser.Lancamento{
		{Centavos: 350000, Categoria: parser.Salario, Tipo: parser.Receita,
			Data: dia(2026, time.July, 5), Forma: parser.Pix, Parcelas: 1},
		{Centavos: 12000, Categoria: parser.Mercado, Tipo: parser.Despesa,
			Data: dia(2026, time.July, 5), Forma: parser.Debito, Parcelas: 1},
		// R$ 300 in 3 credit installments: R$ 100 in July, August and September.
		{Centavos: 30000, Categoria: parser.Casa, Tipo: parser.Despesa,
			Data: dia(2026, time.July, 5), Forma: parser.Credito, Parcelas: 3},
		// Previous month: must not leak into July.
		{Centavos: 1800, Categoria: parser.Transporte, Tipo: parser.Despesa,
			Data: dia(2026, time.June, 10), Forma: parser.Debito, Parcelas: 1},
		// Credit purchase on the closing day: lands in August, not July.
		{Centavos: 20000, Categoria: parser.Lazer, Tipo: parser.Despesa,
			Data: dia(2026, time.July, 28), Forma: parser.Credito, Parcelas: 1},
	}
}

func TestMensal(t *testing.T) {
	casos := []struct {
		nome          string
		mes           fatura.Competencia
		queroReceitas int64
		queroDespesas int64
		queroEconomia int64
	}{
		// July: salary 3500; expenses = groceries 120 (debit, counts on the
		// day) + first installment of the credit purchase (100). The purchase
		// on the 28th is not included: its statement is August's.
		{"july", comp(2026, time.July), 350000, 22000, 328000},
		// August: statements only. Installment 2 of the credit purchase (100)
		// + the purchase made on July's closing day (200).
		{"august", comp(2026, time.August), 0, 30000, -30000},
		// September: only the last installment.
		{"september", comp(2026, time.September), 0, 10000, -10000},
		// June: only the debit ride.
		{"june", comp(2026, time.June), 0, 1800, -1800},
		{"empty month", comp(2026, time.November), 0, 0, 0},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, err := resumo.Mensal(c.mes, lancamentos(), fechamento)
			if err != nil {
				t.Fatalf("Mensal returned unexpected error: %v", err)
			}
			if got.ReceitasCentavos != c.queroReceitas {
				t.Errorf("income = %d, want %d", got.ReceitasCentavos, c.queroReceitas)
			}
			if got.DespesasCentavos != c.queroDespesas {
				t.Errorf("expenses = %d, want %d", got.DespesasCentavos, c.queroDespesas)
			}
			if got.EconomiaCentavos != c.queroEconomia {
				t.Errorf("savings = %d, want %d", got.EconomiaCentavos, c.queroEconomia)
			}
		})
	}
}

// A debit expense counts on the day; a credit one counts on its statement.
// They are two different numbers, and mixing them up is the bug this package
// exists to prevent.
func TestMensal_CreditoContaNaFaturaEDebitoNoDia(t *testing.T) {
	compra := dia(2026, time.July, 29) // after closing day 28

	debito := []parser.Lancamento{{Centavos: 5000, Tipo: parser.Despesa,
		Data: compra, Forma: parser.Debito, Parcelas: 1}}
	credito := []parser.Lancamento{{Centavos: 5000, Tipo: parser.Despesa,
		Data: compra, Forma: parser.Credito, Parcelas: 1}}

	julho, agosto := comp(2026, time.July), comp(2026, time.August)

	if r, _ := resumo.Mensal(julho, debito, fechamento); r.DespesasCentavos != 5000 {
		t.Errorf("debit in July = %d, want 5000", r.DespesasCentavos)
	}
	if r, _ := resumo.Mensal(julho, credito, fechamento); r.DespesasCentavos != 0 {
		t.Errorf("credit in July = %d, want 0: its statement is August's", r.DespesasCentavos)
	}
	if r, _ := resumo.Mensal(agosto, credito, fechamento); r.DespesasCentavos != 5000 {
		t.Errorf("credit in August = %d, want 5000", r.DespesasCentavos)
	}
}

// Income does not go through statements: a salary on the 29th is July income
// even with closing day 28. A credit card is a way of spending, not receiving.
func TestMensal_ReceitaIgnoraFatura(t *testing.T) {
	ls := []parser.Lancamento{{Centavos: 350000, Tipo: parser.Receita,
		Data: dia(2026, time.July, 29), Forma: parser.Credito, Parcelas: 3}}

	got, err := resumo.Mensal(comp(2026, time.July), ls, fechamento)
	if err != nil {
		t.Fatalf("Mensal returned unexpected error: %v", err)
	}
	if got.ReceitasCentavos != 350000 {
		t.Errorf("income = %d, want 350000 (in full, in the month of its date)", got.ReceitasCentavos)
	}
}

func TestMensal_PropagaErroDeFatura(t *testing.T) {
	ls := []parser.Lancamento{{Centavos: 30000, Tipo: parser.Despesa,
		Data: dia(2026, time.July, 5), Forma: parser.Credito, Parcelas: 3}}

	if _, err := resumo.Mensal(comp(2026, time.July), ls, 0); !errors.Is(err, fatura.ErrDiaFechamentoInvalido) {
		t.Errorf("error = %v, want ErrDiaFechamentoInvalido", err)
	}
}

// The per-category breakdown must add up to the expenses total, in every month,
// including installments and a purchase on the closing day. A breakdown that
// does not match the total next to it is worse than no breakdown.
func TestMensal_CategoriasSomamODespesas(t *testing.T) {
	for _, mes := range []fatura.Competencia{
		comp(2026, time.June), comp(2026, time.July), comp(2026, time.August),
		comp(2026, time.September), comp(2026, time.November),
	} {
		r, err := resumo.Mensal(mes, lancamentos(), fechamento)
		if err != nil {
			t.Fatalf("%v: unexpected error: %v", mes, err)
		}
		var soma int64
		for _, c := range r.Categorias {
			soma += c.Centavos
		}
		if soma != r.DespesasCentavos {
			t.Errorf("%v: categories add up to %d, expenses total is %d", mes, soma, r.DespesasCentavos)
		}
	}
}

func TestMensal_CategoriasDoMes(t *testing.T) {
	// July: groceries 120 (debit) + first installment of the home purchase (100).
	// The leisure purchase on the 28th belongs to August's statement.
	r, err := resumo.Mensal(comp(2026, time.July), lancamentos(), fechamento)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []resumo.TotalCategoria{
		{Categoria: parser.Mercado, Centavos: 12000},
		{Categoria: parser.Casa, Centavos: 10000},
	}
	if !slices.Equal(r.Categorias, want) {
		t.Errorf("Categorias = %+v, want %+v (largest first)", r.Categorias, want)
	}
}

func TestMensal_CategoriasEmpateOrdemEstavel(t *testing.T) {
	// Same amount in two categories: order by name, so the screen never
	// reshuffles between two loads of the same month.
	ls := []parser.Lancamento{
		{Centavos: 5000, Categoria: parser.Transporte, Tipo: parser.Despesa,
			Data: dia(2026, time.July, 3), Forma: parser.Pix, Parcelas: 1},
		{Centavos: 5000, Categoria: parser.Alimentacao, Tipo: parser.Despesa,
			Data: dia(2026, time.July, 4), Forma: parser.Pix, Parcelas: 1},
	}

	r, err := resumo.Mensal(comp(2026, time.July), ls, fechamento)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Categorias[0].Categoria != parser.Alimentacao || r.Categorias[1].Categoria != parser.Transporte {
		t.Errorf("tie order = %v, %v; want alimentacao before transporte",
			r.Categorias[0].Categoria, r.Categorias[1].Categoria)
	}
}

func TestMensal_MesSemDespesasNaoTemCategorias(t *testing.T) {
	r, _ := resumo.Mensal(comp(2026, time.November), lancamentos(), fechamento)
	if len(r.Categorias) != 0 {
		t.Errorf("Categorias = %+v, want empty", r.Categorias)
	}
}
