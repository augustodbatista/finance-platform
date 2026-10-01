package fatura_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/fatura"
)

func TestPagar(t *testing.T) {
	hoje := time.Date(2026, time.October, 12, 15, 0, 0, 0, time.UTC)
	setembro := fatura.Competencia{Ano: 2026, Mes: time.September}

	p, err := fatura.Pagar(setembro, dia(2026, time.October, 10), 123456, hoje)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Competencia != setembro || !p.Data.Equal(dia(2026, time.October, 10)) || p.Centavos != 123456 {
		t.Errorf("payment = %+v", p)
	}
}

func TestPagar_Recusa(t *testing.T) {
	hoje := time.Date(2026, time.October, 12, 15, 0, 0, 0, time.UTC)
	setembro := fatura.Competencia{Ano: 2026, Mes: time.September}

	casos := []struct {
		nome     string
		data     time.Time
		centavos int64
		quero    error
	}{
		{"zero amount", hoje, 0, fatura.ErrPagamentoInvalido},
		{"negative amount", hoje, -1, fatura.ErrPagamentoInvalido},
		// "Paid" means it already happened.
		{"date in the future", dia(2026, time.October, 13), 1000, fatura.ErrPagamentoNoFuturo},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if _, err := fatura.Pagar(setembro, c.data, c.centavos, hoje); !errors.Is(err, c.quero) {
				t.Errorf("error = %v, want %v", err, c.quero)
			}
		})
	}
}

// Same calendar rule as bills: a payment dated today is accepted even when its
// midnight, in its own time zone, is a later instant than today's.
func TestPagar_HojeEmOutroFuso(t *testing.T) {
	hoje := time.Date(2026, time.October, 12, 12, 0, 0, 0, time.FixedZone("BRT", -3*60*60))
	utcMenos5 := time.FixedZone("UTC-5", -5*60*60)
	setembro := fatura.Competencia{Ano: 2026, Mes: time.September}

	if _, err := fatura.Pagar(setembro, time.Date(2026, time.October, 12, 0, 0, 0, 0, utcMenos5), 1000, hoje); err != nil {
		t.Errorf("payment dated today (UTC-5) rejected: %v", err)
	}
}

func TestCompetencia_TextoIda_EVolta(t *testing.T) {
	c := fatura.Competencia{Ano: 2026, Mes: time.September}
	if c.String() != "2026-09" {
		t.Errorf("String() = %q, want 2026-09", c.String())
	}
	got, err := fatura.ParseCompetencia("2026-09")
	if err != nil || got != c {
		t.Errorf("ParseCompetencia = %+v, %v; want %+v", got, err, c)
	}
	for _, ruim := range []string{"2026-13", "2026-9", "setembro", ""} {
		if _, err := fatura.ParseCompetencia(ruim); err == nil {
			t.Errorf("ParseCompetencia(%q) should fail", ruim)
		}
	}
}

// The payment is persisted in dados.json: the statement month must be written
// as "2026-09" and read back identically.
func TestPagamento_JSON(t *testing.T) {
	p := fatura.Pagamento{
		Competencia: fatura.Competencia{Ano: 2026, Mes: time.September},
		Data:        dia(2026, time.October, 10),
		Centavos:    123456,
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"competencia":"2026-09"`) {
		t.Errorf("JSON = %s, want competencia as \"2026-09\"", b)
	}

	var volta fatura.Pagamento
	if err := json.Unmarshal(b, &volta); err != nil || volta.Competencia != p.Competencia ||
		!volta.Data.Equal(p.Data) || volta.Centavos != p.Centavos {
		t.Errorf("round trip = %+v, %v; want %+v", volta, err, p)
	}

	if err := json.Unmarshal([]byte(`{"competencia":"2026-13"}`), &volta); err == nil {
		t.Error("an invalid statement month in the file must be an error, not a silent zero")
	}
}
