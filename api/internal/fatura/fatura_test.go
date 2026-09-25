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
		{"before closing stays on this month's statement",
			dia(2026, time.July, 5), 28, 2026, time.July},
		{"day before closing is still this month",
			dia(2026, time.July, 27), 28, 2026, time.July},
		// Product decision: the statement closes at the start of the day, so a
		// purchase made on the closing day already belongs to the next cycle.
		{"on the closing day goes to the next statement",
			dia(2026, time.July, 28), 28, 2026, time.August},
		{"after closing goes to the next statement",
			dia(2026, time.July, 29), 28, 2026, time.August},
		// Year boundary: December + 1 must be January of the following year.
		{"december rolls over to next january",
			dia(2026, time.December, 30), 28, 2027, time.January},
		// Closing on the 31st in a month without a 31st: every day of the month
		// is below 31, so everything lands on that month's statement. No
		// clamping, no special case -- that is why the comparison is by day.
		{"closing on the 31st in february",
			dia(2026, time.February, 28), 31, 2026, time.February},
		{"closing on the 31st, bought on the 31st",
			dia(2026, time.January, 31), 31, 2026, time.February},
		// Closing on the 1st: every day is >= 1, so the statement is always
		// next month's. Odd, but consistent with the rule.
		{"closing on the 1st",
			dia(2026, time.July, 1), 1, 2026, time.August},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, err := fatura.De(c.compra, c.diaFecha)
			if err != nil {
				t.Fatalf("De(%s, %d) returned unexpected error: %v",
					c.compra.Format("2006-01-02"), c.diaFecha, err)
			}
			if got.Ano != c.queroAno || got.Mes != c.queroMes {
				t.Errorf("De(%s, %d) = %d/%s, want %d/%s",
					c.compra.Format("2006-01-02"), c.diaFecha,
					got.Ano, got.Mes, c.queroAno, c.queroMes)
			}
		})
	}
}

func TestDe_DiaFechamentoInvalido(t *testing.T) {
	for _, d := range []int{0, -1, 32, 100} {
		if _, err := fatura.De(dia(2026, time.July, 15), d); !errors.Is(err, fatura.ErrDiaFechamentoInvalido) {
			t.Errorf("De(_, %d) error = %v, want ErrDiaFechamentoInvalido", d, err)
		}
	}
}
