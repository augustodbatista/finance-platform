package dominio_test

import (
	"testing"

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
		// A category that does not exist is not income either.
		dominio.Categoria("inexistente"): dominio.Despesa,
	}
	for c, quero := range casos {
		if got := dominio.TipoDe(c); got != quero {
			t.Errorf("TipoDe(%q) = %q, want %q", c, got, quero)
		}
	}
}
