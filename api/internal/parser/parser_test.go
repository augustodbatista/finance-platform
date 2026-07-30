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
		// Virgula e o separador decimal em pt-BR.
		{"Almoço 42,50", 4250},
		// Um decimal so ainda sao centavos: 42,5 = 42 reais e 50 centavos.
		{"cafe 42,5", 4250},
		// Ponto e separador de milhar, e os dois convivem.
		{"R$ 1.234,56", 123456},
		{"1.234", 123400},
		// Mais de um numero: vale o ultimo (decisao de produto, 30/07/2026).
		{"2 cafés 15", 1500},
		{"3x uber 18", 1800},
		// Valor antes ou depois da descricao da na mesma coisa.
		{"Uber 18", 1800},
		{"18 uber", 1800},
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
