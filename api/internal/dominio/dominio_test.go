package dominio_test

import (
	"errors"
	"testing"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/dominio"
)

func TestTipoDe(t *testing.T) {
	casos := map[dominio.Categoria]dominio.Tipo{
		dominio.Salario:       dominio.Receita,
		dominio.Freelancer:    dominio.Receita,
		dominio.Investimentos: dominio.Receita,
		dominio.Mercado:       dominio.Despesa,
		dominio.Assinaturas:   dominio.Despesa,
		// Outros exists in both lists; with no sign of income it is an expense.
		dominio.Outros: dominio.Despesa,
		// A statement adjustment is money going out (or a refund of it).
		dominio.AjusteFatura: dominio.Despesa,
		// A category that does not exist is not income either.
		dominio.Categoria("inexistente"): dominio.Despesa,
	}
	for c, quero := range casos {
		if got := dominio.TipoDe(c); got != quero {
			t.Errorf("TipoDe(%q) = %q, want %q", c, got, quero)
		}
	}
}

var brt = time.FixedZone("BRT", -3*60*60)

func dia(ano int, mes time.Month, d int) time.Time {
	return time.Date(ano, mes, d, 0, 0, 0, 0, brt)
}

func despesaPix() dominio.Lancamento {
	return dominio.Lancamento{Centavos: 18000, Categoria: dominio.Casa, Tipo: dominio.Despesa,
		Data: dia(2026, time.October, 1), Forma: dominio.Pix, Parcelas: 1}
}

func TestComoConta(t *testing.T) {
	l, err := despesaPix().ComoConta(dia(2026, time.October, 10))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !l.Vencimento.Equal(dia(2026, time.October, 10)) || !l.EConta() || l.Pago() {
		t.Errorf("bill = %+v; want due Oct 10, a bill, not paid", l)
	}
	if despesaPix().EConta() {
		t.Error("a regular entry must not be a bill")
	}
}

func TestComoConta_Recusa(t *testing.T) {
	credito := despesaPix()
	credito.Forma = dominio.Credito
	receita := despesaPix()
	receita.Tipo, receita.Categoria = dominio.Receita, dominio.Salario

	casos := []struct {
		nome  string
		l     dominio.Lancamento
		venc  time.Time
		quero error
	}{
		// Credit purchases are paid through the statement (next slice).
		{"credit purchase", credito, dia(2026, time.October, 10), dominio.ErrContaNoCredito},
		{"income", receita, dia(2026, time.October, 10), dominio.ErrContaReceita},
		{"no due date", despesaPix(), time.Time{}, dominio.ErrSemVencimento},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if _, err := c.l.ComoConta(c.venc); !errors.Is(err, c.quero) {
				t.Errorf("error = %v, want %v", err, c.quero)
			}
		})
	}
}

func TestPagar(t *testing.T) {
	hoje := dia(2026, time.October, 12)
	conta, _ := despesaPix().ComoConta(dia(2026, time.October, 10))

	// Paid late, with a fine: the paid amount is what counts.
	paga, err := conta.Pagar(dia(2026, time.October, 12), 18540, hoje)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !paga.Pago() || !paga.Pagamento.Data.Equal(dia(2026, time.October, 12)) || paga.Pagamento.Centavos != 18540 {
		t.Errorf("paid = %+v", paga.Pagamento)
	}
	if paga.Centavos != 18000 {
		t.Errorf("Centavos = %d, want 18000: the expected amount is kept next to the paid one", paga.Centavos)
	}

	// Paying again corrects the payment instead of failing.
	corrigida, err := paga.Pagar(dia(2026, time.October, 11), 18000, hoje)
	if err != nil || corrigida.Pagamento.Centavos != 18000 || !corrigida.Pagamento.Data.Equal(dia(2026, time.October, 11)) {
		t.Errorf("corrected = %+v, err %v", corrigida.Pagamento, err)
	}

	desfeita := paga.DesfazerPagamento()
	if desfeita.Pago() || !desfeita.EConta() {
		t.Errorf("after undo: paid=%v bill=%v, want unpaid bill", desfeita.Pago(), desfeita.EConta())
	}
}

func TestPagar_Recusa(t *testing.T) {
	hoje := dia(2026, time.October, 12)
	conta, _ := despesaPix().ComoConta(dia(2026, time.October, 10))

	casos := []struct {
		nome     string
		l        dominio.Lancamento
		data     time.Time
		centavos int64
		quero    error
	}{
		{"not a bill", despesaPix(), hoje, 18000, dominio.ErrNaoEConta},
		{"zero amount", conta, hoje, 0, dominio.ErrPagamentoInvalido},
		{"negative amount", conta, hoje, -100, dominio.ErrPagamentoInvalido},
		// "Paid" means it already happened.
		{"date in the future", conta, dia(2026, time.October, 13), 18000, dominio.ErrPagamentoNoFuturo},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if _, err := c.l.Pagar(c.data, c.centavos, hoje); !errors.Is(err, c.quero) {
				t.Errorf("error = %v, want %v", err, c.quero)
			}
		})
	}
}

func TestPagar_HojeEmQualquerHorario(t *testing.T) {
	// "hoje" may carry a time of day; a payment dated today must be accepted.
	agora := time.Date(2026, time.October, 12, 23, 59, 0, 0, brt)
	conta, _ := despesaPix().ComoConta(dia(2026, time.October, 10))
	if _, err := conta.Pagar(dia(2026, time.October, 12), 18000, agora); err != nil {
		t.Errorf("payment dated today rejected: %v", err)
	}
}

func TestVencida(t *testing.T) {
	hoje := time.Date(2026, time.October, 15, 23, 30, 0, 0, brt)
	conta := func(venc time.Time) dominio.Lancamento {
		c, err := despesaPix().ComoConta(venc)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	paga, _ := conta(dia(2026, time.October, 1)).Pagar(dia(2026, time.October, 2), 18000, hoje)

	casos := []struct {
		nome  string
		l     dominio.Lancamento
		quero bool
	}{
		{"due yesterday, unpaid", conta(dia(2026, time.October, 14)), true},
		{"due today", conta(dia(2026, time.October, 15)), false},
		{"due tomorrow", conta(dia(2026, time.October, 16)), false},
		{"overdue but paid", paga, false},
		{"regular entry", despesaPix(), false},
		// Calendar days, not instants: due today at UTC midnight is still
		// today in Brasilia, even though that instant is 3 hours earlier.
		{"due today in another time zone", conta(time.Date(2026, time.October, 15, 0, 0, 0, 0, time.UTC)), false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := c.l.Vencida(hoje); got != c.quero {
				t.Errorf("Vencida = %v, want %v", got, c.quero)
			}
		})
	}
}

// Same calendar rule as Vencida: a payment dated today is accepted even when
// its midnight, in its own time zone, is a later instant than today's midnight
// in Brasilia.
func TestPagar_HojeEmOutroFuso(t *testing.T) {
	hoje := time.Date(2026, time.October, 12, 12, 0, 0, 0, brt)
	utcMenos5 := time.FixedZone("UTC-5", -5*60*60)
	conta, _ := despesaPix().ComoConta(dia(2026, time.October, 10))

	if _, err := conta.Pagar(time.Date(2026, time.October, 12, 0, 0, 0, 0, utcMenos5), 18000, hoje); err != nil {
		t.Errorf("payment dated today (UTC-5) rejected: %v", err)
	}
}
