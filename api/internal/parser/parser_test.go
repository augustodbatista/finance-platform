package parser_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/augustodbatista/finance-platform/api/internal/parser"
)

func TestParse_Erros(t *testing.T) {
	casos := []struct {
		nome    string
		entrada string
		quero   error
	}{
		{"so descricao", "mercado", parser.ErrSemValor},
		{"vazia", "", parser.ErrSemValor},
		{"so espacos", "   ", parser.ErrSemValor},
		{"sem digito algum", "almoço no centro", parser.ErrSemValor},
		// Lancamento de zero nao significa nada e provavelmente e engano.
		{"valor zero", "0 mercado", parser.ErrValorNaoPositivo},
		{"zero com decimais", "0,00 mercado", parser.ErrValorNaoPositivo},
		// 20 digitos estouram int64 em centavos. Tem que doer aqui, nao virar
		// saldo corrompido em silencio la na frente.
		{"estoura int64", "99999999999999999999 mercado", parser.ErrValorInvalido},
		// Limite de tamanho: o parser recebe string livre do usuario, e isso e
		// fronteira de confianca.
		{"entrada longa demais", strings.Repeat("a", 201) + " 10", parser.ErrEntradaLonga},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, err := parser.Parse(c.entrada)
			if !errors.Is(err, c.quero) {
				t.Errorf("Parse(%.30q) erro = %v, quero %v", c.entrada, err, c.quero)
			}
		})
	}
}

func TestParse_LimiteDeTamanho(t *testing.T) {
	// Exatamente no limite tem que passar; um caractere alem, nao.
	noLimite := strings.Repeat("a", parser.MaxEntrada-3) + " 10"
	if _, err := parser.Parse(noLimite); err != nil {
		t.Errorf("entrada de %d chars (limite %d) devia passar, deu %v",
			len(noLimite), parser.MaxEntrada, err)
	}

	alem := strings.Repeat("a", parser.MaxEntrada) + " 10"
	if _, err := parser.Parse(alem); !errors.Is(err, parser.ErrEntradaLonga) {
		t.Errorf("entrada de %d chars devia dar ErrEntradaLonga, deu %v", len(alem), err)
	}
}

func TestParse_SinalNegativoEIgnorado(t *testing.T) {
	// O sinal e redundante: a direcao do dinheiro ja esta em Tipo. Entao
	// "-5 mercado" e uma despesa de R$ 5,00, nao um erro nem um valor negativo.
	got, err := parser.Parse("-5 mercado")
	if err != nil {
		t.Fatalf("Parse retornou erro inesperado: %v", err)
	}
	if got.Centavos != 500 {
		t.Errorf("Centavos = %d, quero 500", got.Centavos)
	}
	if got.Tipo != parser.Despesa {
		t.Errorf("Tipo = %q, quero %q", got.Tipo, parser.Despesa)
	}
}

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
		// Mais de duas casas e digitacao errada: trunca, nao arredonda, para
		// nao inventar centavo que o usuario nao digitou.
		{"cafe 42,555", 4255},
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
	casos := []struct {
		entrada string
		quero   parser.Tipo
	}{
		{"120 mercado", parser.Despesa},
		{"Uber 18", parser.Despesa},
		// Categoria de receita define o tipo; o usuario nao precisa dizer.
		{"Salário 3500", parser.Receita},
		{"freela 800", parser.Receita},
		{"dividendo 120", parser.Receita},
		// Outros e ambigua por natureza: sem sinal de receita, e despesa,
		// que e a esmagadora maioria dos lancamentos.
		{"xyzabc 30", parser.Despesa},
	}

	for _, c := range casos {
		t.Run(c.entrada, func(t *testing.T) {
			got, err := parser.Parse(c.entrada)
			if err != nil {
				t.Fatalf("Parse(%q) retornou erro inesperado: %v", c.entrada, err)
			}
			if got.Tipo != c.quero {
				t.Errorf("Tipo = %q, quero %q", got.Tipo, c.quero)
			}
		})
	}
}

func TestParse_CategoriaReceita(t *testing.T) {
	casos := []struct {
		entrada string
		quero   parser.Categoria
	}{
		{"Salário 3500", parser.Salario},
		{"freela 800", parser.Freelancer},
		{"dividendo 120", parser.Investimentos},
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
