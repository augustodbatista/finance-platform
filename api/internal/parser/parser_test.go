package parser_test

import (
	"testing"

	"github.com/augustodbatista/finance-platform/api/internal/parser"
)

func TestParse_ValorECategoria(t *testing.T) {
	got, err := parser.Parse("120 mercado")
	if err != nil {
		t.Fatalf("Parse(%q) retornou erro inesperado: %v", "120 mercado", err)
	}

	if got.Centavos != 12000 {
		t.Errorf("Centavos = %d, quero 12000", got.Centavos)
	}
	if got.Categoria != parser.Mercado {
		t.Errorf("Categoria = %q, quero %q", got.Categoria, parser.Mercado)
	}
	if got.Tipo != parser.Despesa {
		t.Errorf("Tipo = %q, quero %q", got.Tipo, parser.Despesa)
	}
}
