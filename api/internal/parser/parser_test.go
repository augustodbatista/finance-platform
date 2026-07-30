package parser_test

import (
	"testing"

	"github.com/augustodbatista/finance-platform/api/internal/parser"
)

func TestParse_Valor(t *testing.T) {
	casos := []struct {
		entrada string
		quero   int64
	}{
		{"120 mercado", 12000},
	}

	for _, c := range casos {
		t.Run(c.entrada, func(t *testing.T) {
			got, err := parser.Parse(c.entrada)
			if err != nil {
				t.Fatalf("Parse(%q) retornou erro inesperado: %v", c.entrada, err)
			}
			if got.Centavos != c.quero {
				t.Errorf("Centavos = %d, quero %d", got.Centavos, c.quero)
			}
		})
	}
}

func TestParse_Categoria(t *testing.T) {
	casos := []struct {
		entrada string
		quero   parser.Categoria
	}{
		// A propria palavra da categoria e um termo valido.
		{"120 mercado", parser.Mercado},
		// Termo de marca mapeia para a categoria.
		{"59 netflix", parser.Assinaturas},
		{"Uber 18", parser.Transporte},
		// Caixa e acento nao podem mudar o resultado.
		{"ALMOÇO 10", parser.Alimentacao},
		{"almoco 10", parser.Alimentacao},
		{"Almoço 10", parser.Alimentacao},
		// Termo desconhecido cai em Outros: atrito zero vale mais que
		// precisao de categoria, entao isto nao e erro.
		{"xyzabc 30", parser.Outros},
	}

	for _, c := range casos {
		t.Run(c.entrada, func(t *testing.T) {
			got, err := parser.Parse(c.entrada)
			if err != nil {
				t.Fatalf("Parse(%q) retornou erro inesperado: %v", c.entrada, err)
			}
			if got.Categoria != c.quero {
				t.Errorf("Categoria = %q, quero %q", got.Categoria, c.quero)
			}
		})
	}
}

func TestParse_Tipo(t *testing.T) {
	got, err := parser.Parse("120 mercado")
	if err != nil {
		t.Fatalf("Parse retornou erro inesperado: %v", err)
	}
	if got.Tipo != parser.Despesa {
		t.Errorf("Tipo = %q, quero %q", got.Tipo, parser.Despesa)
	}
}
