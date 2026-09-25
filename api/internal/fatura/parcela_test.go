package fatura_test

import (
	"errors"
	"testing"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/fatura"
)

func TestDividir_ValoresECompetencias(t *testing.T) {
	// R$ 300.00 in 3 installments, bought on July 5 with closing day 28:
	// divides evenly and lands on July, August and September.
	got, err := fatura.Dividir(30000, 3, dia(2026, time.July, 5), 28)
	if err != nil {
		t.Fatalf("Dividir returned unexpected error: %v", err)
	}

	quero := []fatura.Parcela{
		{Numero: 1, Centavos: 10000, Competencia: fatura.Competencia{Ano: 2026, Mes: time.July}},
		{Numero: 2, Centavos: 10000, Competencia: fatura.Competencia{Ano: 2026, Mes: time.August}},
		{Numero: 3, Centavos: 10000, Competencia: fatura.Competencia{Ano: 2026, Mes: time.September}},
	}
	if len(got) != len(quero) {
		t.Fatalf("len = %d, want %d", len(got), len(quero))
	}
	for i := range quero {
		if got[i] != quero[i] {
			t.Errorf("installment %d = %+v, want %+v", i+1, got[i], quero[i])
		}
	}
}

func TestDividir_SobraVaiNaPrimeira(t *testing.T) {
	// R$ 100.00 in 3 installments is 33.33 three times with 1 cent left over.
	// Brazilian issuers put the remainder on the first installment.
	got, err := fatura.Dividir(10000, 3, dia(2026, time.July, 5), 28)
	if err != nil {
		t.Fatalf("Dividir returned unexpected error: %v", err)
	}

	quero := []int64{3334, 3333, 3333}
	for i, c := range quero {
		if got[i].Centavos != c {
			t.Errorf("installment %d = %d cents, want %d", i+1, got[i].Centavos, c)
		}
	}
}

// The invariant that matters: no cent may vanish or appear out of nowhere, for
// any total and any number of installments.
func TestDividir_SomaSempreFechaComOTotal(t *testing.T) {
	totais := []int64{1, 2, 7, 99, 100, 101, 3333, 10000, 12345, 99999, 1_000_000_007}
	parcelas := []int{1, 2, 3, 4, 6, 7, 12, 13, 24, 99}

	for _, total := range totais {
		for _, n := range parcelas {
			if total < int64(n) {
				continue
			}
			ps, err := fatura.Dividir(total, n, dia(2026, time.July, 5), 28)
			if err != nil {
				t.Fatalf("Dividir(%d, %d) returned unexpected error: %v", total, n, err)
			}

			var soma int64
			for _, p := range ps {
				if p.Centavos <= 0 {
					t.Errorf("Dividir(%d, %d): installment %d has %d cents", total, n, p.Numero, p.Centavos)
				}
				soma += p.Centavos
			}
			if soma != total {
				t.Errorf("Dividir(%d, %d): installments add up to %d", total, n, soma)
			}
		}
	}
}

func TestDividir_ViradaDeAno(t *testing.T) {
	// Bought on December 15 with closing day 28: the first installment lands
	// in December and the rest cross into the new year.
	got, err := fatura.Dividir(30000, 3, dia(2026, time.December, 15), 28)
	if err != nil {
		t.Fatalf("Dividir returned unexpected error: %v", err)
	}

	quero := []fatura.Competencia{
		{Ano: 2026, Mes: time.December},
		{Ano: 2027, Mes: time.January},
		{Ano: 2027, Mes: time.February},
	}
	for i, c := range quero {
		if got[i].Competencia != c {
			t.Errorf("installment %d on statement %+v, want %+v", i+1, got[i].Competencia, c)
		}
	}
}

func TestDividir_PrimeiraParcelaSegueARegraDoFechamento(t *testing.T) {
	// Bought on the closing day itself: the first installment already lands on
	// the next statement, and the rest follow from there.
	got, err := fatura.Dividir(20000, 2, dia(2026, time.July, 28), 28)
	if err != nil {
		t.Fatalf("Dividir returned unexpected error: %v", err)
	}
	if got[0].Competencia.Mes != time.August {
		t.Errorf("first installment in %s, want August", got[0].Competencia.Mes)
	}
	if got[1].Competencia.Mes != time.September {
		t.Errorf("second installment in %s, want September", got[1].Competencia.Mes)
	}
}

func TestDividir_Erros(t *testing.T) {
	compra := dia(2026, time.July, 5)
	casos := []struct {
		nome     string
		total    int64
		parcelas int
		diaFecha int
		quero    error
	}{
		{"zero installments", 10000, 0, 28, fatura.ErrParcelasInvalidas},
		{"negative installments", 10000, -3, 28, fatura.ErrParcelasInvalidas},
		{"above the ceiling", 10000, fatura.MaxParcelas + 1, 28, fatura.ErrParcelasInvalidas},
		// 2 cents in 3 installments would produce a zero-cent installment,
		// which describes nothing.
		{"total below number of installments", 2, 3, 28, fatura.ErrTotalInsuficiente},
		{"zero total", 0, 1, 28, fatura.ErrTotalInsuficiente},
		{"negative total", -100, 1, 28, fatura.ErrTotalInsuficiente},
		{"invalid closing day", 10000, 3, 0, fatura.ErrDiaFechamentoInvalido},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, err := fatura.Dividir(c.total, c.parcelas, compra, c.diaFecha)
			if !errors.Is(err, c.quero) {
				t.Errorf("Dividir(%d, %d, _, %d) error = %v, want %v",
					c.total, c.parcelas, c.diaFecha, err, c.quero)
			}
		})
	}
}
