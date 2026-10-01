package resumo_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/dominio"
	"github.com/augustodbatista/finance-platform/api/internal/fatura"
	"github.com/augustodbatista/finance-platform/api/internal/resumo"
)

const fechamento = 28

var brt = time.FixedZone("BRT", -3*60*60)

func dia(ano int, mes time.Month, d int) time.Time {
	return time.Date(ano, mes, d, 0, 0, 0, 0, time.UTC)
}

func comp(ano int, mes time.Month) fatura.Competencia {
	return fatura.Competencia{Ano: ano, Mes: mes}
}

// lancamentos builds the same set for every test: a month with income, a debit
// expense, a credit card installment purchase, an entry from the previous month
// and a purchase made on the closing day itself.
func lancamentos() []dominio.Lancamento {
	return []dominio.Lancamento{
		{Centavos: 350000, Categoria: dominio.Salario, Tipo: dominio.Receita,
			Data: dia(2026, time.July, 5), Forma: dominio.Pix, Parcelas: 1},
		{Centavos: 12000, Categoria: dominio.Mercado, Tipo: dominio.Despesa,
			Data: dia(2026, time.July, 5), Forma: dominio.Debito, Parcelas: 1},
		// R$ 300 in 3 credit installments: R$ 100 in July, August and September.
		{Centavos: 30000, Categoria: dominio.Casa, Tipo: dominio.Despesa,
			Data: dia(2026, time.July, 5), Forma: dominio.Credito, Parcelas: 3},
		// Previous month: must not leak into July.
		{Centavos: 1800, Categoria: dominio.Transporte, Tipo: dominio.Despesa,
			Data: dia(2026, time.June, 10), Forma: dominio.Debito, Parcelas: 1},
		// Credit purchase on the closing day: lands in August, not July.
		{Centavos: 20000, Categoria: dominio.Lazer, Tipo: dominio.Despesa,
			Data: dia(2026, time.July, 28), Forma: dominio.Credito, Parcelas: 1},
	}
}

// pagasNoProprioMes pays each statement of lancamentos() on the last day of its
// own month, for its exact total: July 10000 (installment 1 of the home
// purchase), August 30000 (installment 2 + the purchase on the closing day),
// September 10000 (installment 3). Under it, the statement month and the payment
// month coincide, so tests about installments and categories read as before.
func pagasNoProprioMes(t *testing.T) []fatura.Pagamento {
	t.Helper()
	pagar := func(mes time.Month, ultimoDia int, centavos int64) fatura.Pagamento {
		p, err := fatura.Pagar(comp(2026, mes), dia(2026, mes, ultimoDia), centavos, dia(2026, time.December, 31))
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	return []fatura.Pagamento{
		pagar(time.July, 31, 10000),
		pagar(time.August, 31, 30000),
		pagar(time.September, 30, 10000),
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
			got, err := resumo.Mensal(c.mes, lancamentos(), pagasNoProprioMes(t), fechamento)
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

// A debit expense is spent on the day. A credit purchase is pending ("a
// pagar") on its statement until the statement is paid, and then it is spent in
// the month of the payment. Mixing these up is the bug this package exists to
// prevent.
func TestMensal_CreditoAPagarAtePagarAFatura(t *testing.T) {
	compra := dia(2026, time.July, 29) // after closing day 28: August's statement

	debito := []dominio.Lancamento{{Centavos: 5000, Categoria: dominio.Lazer, Tipo: dominio.Despesa,
		Data: compra, Forma: dominio.Debito, Parcelas: 1}}
	credito := []dominio.Lancamento{{Centavos: 5000, Categoria: dominio.Lazer, Tipo: dominio.Despesa,
		Data: compra, Forma: dominio.Credito, Parcelas: 1}}
	julho, agosto, setembro := comp(2026, time.July), comp(2026, time.August), comp(2026, time.September)

	if r, _ := resumo.Mensal(julho, debito, nil, fechamento); r.DespesasCentavos != 5000 {
		t.Errorf("debit in July = %d, want 5000", r.DespesasCentavos)
	}

	// Statement not paid: nothing spent, pending in the statement month.
	if r, _ := resumo.Mensal(julho, credito, nil, fechamento); r.DespesasCentavos != 0 || r.APagarCentavos != 0 {
		t.Errorf("credit in July = spent %d, pending %d; want 0 and 0 (its statement is August's)",
			r.DespesasCentavos, r.APagarCentavos)
	}
	if r, _ := resumo.Mensal(agosto, credito, nil, fechamento); r.DespesasCentavos != 0 || r.APagarCentavos != 5000 {
		t.Errorf("unpaid August statement = spent %d, pending %d; want 0 and 5000", r.DespesasCentavos, r.APagarCentavos)
	}

	// Paid on September 5th: spent in September, no longer pending in August.
	pago, err := fatura.Pagar(agosto, dia(2026, time.September, 5), 5000, dia(2026, time.December, 31))
	if err != nil {
		t.Fatal(err)
	}
	pagas := []fatura.Pagamento{pago}
	if r, _ := resumo.Mensal(agosto, credito, pagas, fechamento); r.DespesasCentavos != 0 || r.APagarCentavos != 0 {
		t.Errorf("paid August statement, viewed in August = spent %d, pending %d; want 0 and 0",
			r.DespesasCentavos, r.APagarCentavos)
	}
	r, _ := resumo.Mensal(setembro, credito, pagas, fechamento)
	if r.DespesasCentavos != 5000 || len(r.Categorias) != 1 || r.Categorias[0].Categoria != dominio.Lazer {
		t.Errorf("September = spent %d, categories %+v; want 5000 under lazer", r.DespesasCentavos, r.Categorias)
	}
}

// Income does not go through statements: a salary on the 29th is July income
// even with closing day 28. A credit card is a way of spending, not receiving.
func TestMensal_ReceitaIgnoraFatura(t *testing.T) {
	ls := []dominio.Lancamento{{Centavos: 350000, Tipo: dominio.Receita,
		Data: dia(2026, time.July, 29), Forma: dominio.Credito, Parcelas: 3}}

	got, err := resumo.Mensal(comp(2026, time.July), ls, nil, fechamento)
	if err != nil {
		t.Fatalf("Mensal returned unexpected error: %v", err)
	}
	if got.ReceitasCentavos != 350000 {
		t.Errorf("income = %d, want 350000 (in full, in the month of its date)", got.ReceitasCentavos)
	}
}

func TestMensal_PropagaErroDeFatura(t *testing.T) {
	ls := []dominio.Lancamento{{Centavos: 30000, Tipo: dominio.Despesa,
		Data: dia(2026, time.July, 5), Forma: dominio.Credito, Parcelas: 3}}

	if _, err := resumo.Mensal(comp(2026, time.July), ls, nil, 0); !errors.Is(err, fatura.ErrDiaFechamentoInvalido) {
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
		for _, pagas := range [][]fatura.Pagamento{nil, pagasNoProprioMes(t)} {
			r, err := resumo.Mensal(mes, lancamentos(), pagas, fechamento)
			if err != nil {
				t.Fatalf("%v: unexpected error: %v", mes, err)
			}
			var soma int64
			for _, c := range r.Categorias {
				soma += c.Centavos
			}
			if soma != r.DespesasCentavos {
				t.Errorf("%v (%d paid statements): categories add up to %d, expenses total is %d",
					mes, len(pagas), soma, r.DespesasCentavos)
			}
		}
	}
}

func TestMensal_CategoriasDoMes(t *testing.T) {
	// July: groceries 120 (debit) + first installment of the home purchase (100).
	// The leisure purchase on the 28th belongs to August's statement.
	r, err := resumo.Mensal(comp(2026, time.July), lancamentos(), pagasNoProprioMes(t), fechamento)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []resumo.TotalCategoria{
		{Categoria: dominio.Mercado, Centavos: 12000},
		{Categoria: dominio.Casa, Centavos: 10000},
	}
	if !slices.Equal(r.Categorias, want) {
		t.Errorf("Categorias = %+v, want %+v (largest first)", r.Categorias, want)
	}
}

func TestMensal_CategoriasEmpateOrdemEstavel(t *testing.T) {
	// Same amount in two categories: order by name, so the screen never
	// reshuffles between two loads of the same month.
	ls := []dominio.Lancamento{
		{Centavos: 5000, Categoria: dominio.Transporte, Tipo: dominio.Despesa,
			Data: dia(2026, time.July, 3), Forma: dominio.Pix, Parcelas: 1},
		{Centavos: 5000, Categoria: dominio.Alimentacao, Tipo: dominio.Despesa,
			Data: dia(2026, time.July, 4), Forma: dominio.Pix, Parcelas: 1},
	}

	r, err := resumo.Mensal(comp(2026, time.July), ls, nil, fechamento)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Categorias[0].Categoria != dominio.Alimentacao || r.Categorias[1].Categoria != dominio.Transporte {
		t.Errorf("tie order = %v, %v; want alimentacao before transporte",
			r.Categorias[0].Categoria, r.Categorias[1].Categoria)
	}
}

func TestMensal_MesSemDespesasNaoTemCategorias(t *testing.T) {
	r, _ := resumo.Mensal(comp(2026, time.November), lancamentos(), nil, fechamento)
	if len(r.Categorias) != 0 {
		t.Errorf("Categorias = %+v, want empty", r.Categorias)
	}
}

func conta(t *testing.T, centavos int64, cat dominio.Categoria, vencimento time.Time) dominio.Lancamento {
	t.Helper()
	l := dominio.Lancamento{Centavos: centavos, Categoria: cat, Tipo: dominio.Despesa,
		Data: dia(2026, time.September, 1), Forma: dominio.Pix, Parcelas: 1}
	c, err := l.ComoConta(vencimento)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func paga(t *testing.T, l dominio.Lancamento, data time.Time, centavos int64) dominio.Lancamento {
	t.Helper()
	p, err := l.Pagar(data, centavos, dia(2026, time.December, 31))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Expenses are what left the pocket: a bill counts when paid, in the month of
// the payment and for the amount paid. Until then it is "a pagar" in the month
// it is due.
func TestMensal_Contas(t *testing.T) {
	luz := conta(t, 18000, dominio.Casa, dia(2026, time.September, 10))
	// Due in September, paid late in October with a fine.
	luzPaga := paga(t, luz, dia(2026, time.October, 2), 18540)
	internet := conta(t, 10000, dominio.Casa, dia(2026, time.September, 20)) // still unpaid
	ls := []dominio.Lancamento{luzPaga, internet}

	setembro, _ := resumo.Mensal(comp(2026, time.September), ls, nil, fechamento)
	if setembro.DespesasCentavos != 0 || setembro.APagarCentavos != 10000 {
		t.Errorf("September: expenses %d, a pagar %d; want 0 and 10000 (only the unpaid bill is pending)",
			setembro.DespesasCentavos, setembro.APagarCentavos)
	}

	outubro, _ := resumo.Mensal(comp(2026, time.October), ls, nil, fechamento)
	if outubro.DespesasCentavos != 18540 || outubro.APagarCentavos != 0 {
		t.Errorf("October: expenses %d, a pagar %d; want 18540 (paid amount, payment month) and 0",
			outubro.DespesasCentavos, outubro.APagarCentavos)
	}
	if len(outubro.Categorias) != 1 || outubro.Categorias[0].Centavos != 18540 {
		t.Errorf("October categories = %+v, want casa 18540", outubro.Categorias)
	}
	if outubro.EconomiaCentavos != -18540 {
		t.Errorf("October savings = %d, want -18540: pending bills do not count as spent", outubro.EconomiaCentavos)
	}
}

// A regular entry -- every entry logged before bills existed -- keeps exactly
// its old behavior: spent on its date, for its amount, never "a pagar".
func TestMensal_LancamentoComumNaoMuda(t *testing.T) {
	for _, mes := range []fatura.Competencia{comp(2026, time.June), comp(2026, time.July), comp(2026, time.August)} {
		r, err := resumo.Mensal(mes, lancamentos(), pagasNoProprioMes(t), fechamento)
		if err != nil {
			t.Fatal(err)
		}
		if r.APagarCentavos != 0 {
			t.Errorf("%v: a pagar = %d, want 0 with no bills", mes, r.APagarCentavos)
		}
	}
}

func TestVencidas(t *testing.T) {
	hoje := time.Date(2026, time.October, 15, 14, 0, 0, 0, brt)
	ls := []dominio.Lancamento{
		conta(t, 18000, dominio.Casa, dia(2026, time.September, 10)),                                      // overdue
		conta(t, 5000, dominio.Saude, dia(2026, time.October, 14)),                                        // overdue (yesterday)
		conta(t, 7000, dominio.Lazer, dia(2026, time.October, 15)),                                        // due today: not yet
		conta(t, 9000, dominio.Educacao, dia(2026, time.October, 30)),                                     // future
		paga(t, conta(t, 3000, dominio.Casa, dia(2026, time.August, 1)), dia(2026, time.August, 1), 3000), // paid
		{Centavos: 4000, Categoria: dominio.Mercado, Tipo: dominio.Despesa, Data: dia(2026, time.September, 1),
			Forma: dominio.Pix, Parcelas: 1}, // regular entry, not a bill
	}

	qtd, centavos := resumo.Vencidas(ls, hoje)
	if qtd != 2 || centavos != 23000 {
		t.Errorf("Vencidas = %d bills, %d cents; want 2 and 23000", qtd, centavos)
	}
}

func pagarFatura(t *testing.T, c fatura.Competencia, data time.Time, centavos int64) fatura.Pagamento {
	t.Helper()
	p, err := fatura.Pagar(c, data, centavos, dia(2026, time.December, 31))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// A statement paid for a different amount than its installments add up to:
// the difference is its own category line, so the categories still add up to
// the expenses total and nothing is hidden inside "lazer" or "casa".
func TestMensal_AjusteDaFatura(t *testing.T) {
	// August's statement: installment 2 of the home purchase (100) + the
	// leisure purchase on July's closing day (200) = 300.
	agosto := comp(2026, time.August)
	casos := []struct {
		nome   string
		pago   int64
		ajuste int64
	}{
		{"interest or unlogged purchase", 30500, 500},
		{"discount or refund", 29000, -1000},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			pagas := []fatura.Pagamento{pagarFatura(t, agosto, dia(2026, time.September, 5), c.pago)}
			r, err := resumo.Mensal(comp(2026, time.September), lancamentos(), pagas, fechamento)
			if err != nil {
				t.Fatal(err)
			}
			if r.DespesasCentavos != c.pago {
				t.Errorf("expenses = %d, want %d (what was paid)", r.DespesasCentavos, c.pago)
			}
			var ajuste, soma int64
			for _, tc := range r.Categorias {
				soma += tc.Centavos
				if tc.Categoria == dominio.AjusteFatura {
					ajuste = tc.Centavos
				}
			}
			if ajuste != c.ajuste || soma != r.DespesasCentavos {
				t.Errorf("ajuste = %d (want %d), categories add up to %d (want %d)",
					ajuste, c.ajuste, soma, r.DespesasCentavos)
			}
		})
	}

	// Paid exactly: no adjustment line at all.
	pagas := []fatura.Pagamento{pagarFatura(t, agosto, dia(2026, time.September, 5), 30000)}
	r, _ := resumo.Mensal(comp(2026, time.September), lancamentos(), pagas, fechamento)
	for _, tc := range r.Categorias {
		if tc.Categoria == dominio.AjusteFatura {
			t.Errorf("exact payment produced an adjustment line: %+v", r.Categorias)
		}
	}
}

// A payment whose statement has no installments any more (the purchases were
// deleted after paying) still left the pocket: all of it is adjustment, in the
// payment month. Dropping it would make money vanish from the dashboard.
func TestMensal_PagamentoSemParcelas(t *testing.T) {
	pagas := []fatura.Pagamento{pagarFatura(t, comp(2026, time.March), dia(2026, time.April, 2), 7000)}
	r, err := resumo.Mensal(comp(2026, time.April), nil, pagas, fechamento)
	if err != nil {
		t.Fatal(err)
	}
	want := []resumo.TotalCategoria{{Categoria: dominio.AjusteFatura, Centavos: 7000}}
	if r.DespesasCentavos != 7000 || !slices.Equal(r.Categorias, want) {
		t.Errorf("expenses %d, categories %+v; want 7000 all as ajuste_fatura", r.DespesasCentavos, r.Categorias)
	}
}

func TestFaturas(t *testing.T) {
	julho, agosto, setembro, marco := comp(2026, time.July), comp(2026, time.August),
		comp(2026, time.September), comp(2026, time.March)
	pagoJulho := pagarFatura(t, julho, dia(2026, time.August, 5), 10000)
	orfao := pagarFatura(t, marco, dia(2026, time.April, 2), 7000)

	got, err := resumo.Faturas(lancamentos(), []fatura.Pagamento{pagoJulho, orfao}, fechamento)
	if err != nil {
		t.Fatal(err)
	}
	want := []resumo.Fatura{
		{Competencia: setembro, Centavos: 10000},
		{Competencia: agosto, Centavos: 30000},
		{Competencia: julho, Centavos: 10000, Pagamento: &pagoJulho},
		// Listed even with no installments: otherwise its payment could never
		// be seen or undone, while still counting as an expense.
		{Competencia: marco, Centavos: 0, Pagamento: &orfao},
	}
	if len(got) != len(want) {
		t.Fatalf("Faturas = %+v, want %+v", got, want)
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Competencia != w.Competencia || g.Centavos != w.Centavos || (g.Pagamento == nil) != (w.Pagamento == nil) ||
			(g.Pagamento != nil && *g.Pagamento != *w.Pagamento) {
			t.Errorf("Faturas[%d] = %+v, want %+v (newest first)", i, g, w)
		}
	}

	if _, err := resumo.Faturas(lancamentos(), nil, 0); !errors.Is(err, fatura.ErrDiaFechamentoInvalido) {
		t.Errorf("error = %v, want ErrDiaFechamentoInvalido", err)
	}
}
