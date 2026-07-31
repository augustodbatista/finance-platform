package fatura_test

import (
	"errors"
	"testing"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/fatura"
)

func dia(ano int, mes time.Month, d int) time.Time {
	return time.Date(ano, mes, d, 0, 0, 0, 0, time.UTC)
}

func TestDe(t *testing.T) {
	casos := []struct {
		nome     string
		compra   time.Time
		diaFecha int
		queroAno int
		queroMes time.Month
	}{
		{"antes do fechamento fica na fatura do mes",
			dia(2026, time.July, 5), 28, 2026, time.July},
		{"vespera do fechamento ainda e deste mes",
			dia(2026, time.July, 27), 28, 2026, time.July},
		// Decisao do Augusto: a fatura fecha no inicio do dia, entao a compra
		// feita no proprio dia do fechamento ja pertence ao ciclo seguinte.
		{"no dia do fechamento vai para a proxima",
			dia(2026, time.July, 28), 28, 2026, time.August},
		{"depois do fechamento vai para a proxima",
			dia(2026, time.July, 29), 28, 2026, time.August},
		// Virada de ano: dezembro + 1 tem que dar janeiro do ano seguinte.
		{"dezembro vira janeiro do ano seguinte",
			dia(2026, time.December, 30), 28, 2027, time.January},
		// Fechamento 31 em mes que nao tem dia 31: todo dia do mes e menor que
		// 31, entao tudo cai na fatura do proprio mes. Sem clamp, sem caso
		// especial -- e por isso que a comparacao e por numero do dia.
		{"fechamento 31 em fevereiro",
			dia(2026, time.February, 28), 31, 2026, time.February},
		{"fechamento 31 no dia 31",
			dia(2026, time.January, 31), 31, 2026, time.February},
		// Fechamento no dia 1: todo dia e >= 1, entao a fatura e sempre a do
		// mes seguinte. Estranho, mas consistente com a regra.
		{"fechamento no dia 1",
			dia(2026, time.July, 1), 1, 2026, time.August},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, err := fatura.De(c.compra, c.diaFecha)
			if err != nil {
				t.Fatalf("De(%s, %d) retornou erro inesperado: %v",
					c.compra.Format("02/01/2006"), c.diaFecha, err)
			}
			if got.Ano != c.queroAno || got.Mes != c.queroMes {
				t.Errorf("De(%s, %d) = %d/%s, quero %d/%s",
					c.compra.Format("02/01/2006"), c.diaFecha,
					got.Ano, got.Mes, c.queroAno, c.queroMes)
			}
		})
	}
}

func TestDe_DiaFechamentoInvalido(t *testing.T) {
	for _, d := range []int{0, -1, 32, 100} {
		if _, err := fatura.De(dia(2026, time.July, 15), d); !errors.Is(err, fatura.ErrDiaFechamentoInvalido) {
			t.Errorf("De(_, %d) erro = %v, quero ErrDiaFechamentoInvalido", d, err)
		}
	}
}
