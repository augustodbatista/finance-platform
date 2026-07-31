package parser_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/parser"
)

// agora e o relogio fixo dos testes. Parse recebe o relogio em vez de chamar
// time.Now() por dentro: senao "ontem" mudaria de valor conforme o dia em que o
// teste rodasse, e um teste que depende do calendario nao e teste.
var agora = time.Date(2026, time.July, 30, 14, 30, 0, 0, time.UTC)

func TestParse_Data(t *testing.T) {
	casos := []struct {
		nome    string
		entrada string
		relogio time.Time
		quero   time.Time
	}{
		{"sem mencao e hoje", "mercado 120", agora,
			time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)},
		{"hoje explicito", "mercado 120 hoje", agora,
			time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)},
		{"ontem", "mercado 120 ontem", agora,
			time.Date(2026, time.July, 29, 0, 0, 0, 0, time.UTC)},
		{"ontem em caixa alta", "mercado 120 ONTEM", agora,
			time.Date(2026, time.July, 29, 0, 0, 0, 0, time.UTC)},
		{"dd/mm", "mercado 120 15/03", agora,
			time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC)},
		{"dd/mm/aaaa", "mercado 120 15/03/2024", agora,
			time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC)},
		{"dd/mm/aa", "mercado 120 15/03/25", agora,
			time.Date(2025, time.March, 15, 0, 0, 0, 0, time.UTC)},
		// dd/mm sem ano resolve para a ocorrencia passada mais recente:
		// lancamento atrasado e comum, lancamento futuro nao.
		{"dd/mm que ainda nao chegou neste ano", "mercado 120 20/12",
			time.Date(2026, time.January, 15, 9, 0, 0, 0, time.UTC),
			time.Date(2025, time.December, 20, 0, 0, 0, 0, time.UTC)},
		// Ontem atravessando a virada do mes.
		{"ontem no dia 1", "mercado 120 ontem",
			time.Date(2026, time.March, 1, 8, 0, 0, 0, time.UTC),
			time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC)},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, err := parser.Parse(c.entrada, c.relogio)
			if err != nil {
				t.Fatalf("Parse(%q) retornou erro inesperado: %v", c.entrada, err)
			}
			if !got.Data.Equal(c.quero) {
				t.Errorf("Data = %s, quero %s",
					got.Data.Format(time.RFC3339), c.quero.Format(time.RFC3339))
			}
		})
	}
}

// A regra "vale o ultimo numero" colide com a data: sem remover o token antes
// de extrair o valor, "mercado 120 15/03" viraria R$ 0,03. Erro silencioso
// sobre dinheiro e o modo de falha que este projeto trata como inaceitavel.
func TestParse_DataNaoRoubaOValor(t *testing.T) {
	casos := []struct {
		entrada string
		quero   int64
	}{
		{"mercado 120 15/03", 12000},
		{"mercado 120 15/03/2024", 12000},
		{"mercado 120 15/03/25", 12000},
		{"15/03 mercado 120", 12000},
		{"mercado 120 ontem", 12000},
		{"ontem mercado 42,50", 4250},
	}

	for _, c := range casos {
		t.Run(c.entrada, func(t *testing.T) {
			got, err := parser.Parse(c.entrada, agora)
			if err != nil {
				t.Fatalf("Parse(%q) retornou erro inesperado: %v", c.entrada, err)
			}
			if got.Centavos != c.quero {
				t.Errorf("Centavos = %d, quero %d", got.Centavos, c.quero)
			}
		})
	}
}

func TestParse_DataInvalida(t *testing.T) {
	casos := []string{
		"mercado 120 30/02",
		"mercado 120 31/04",
		"mercado 120 15/13",
		"mercado 120 00/01",
	}

	for _, entrada := range casos {
		t.Run(entrada, func(t *testing.T) {
			if _, err := parser.Parse(entrada, agora); !errors.Is(err, parser.ErrDataInvalida) {
				t.Errorf("Parse(%q) erro = %v, quero ErrDataInvalida", entrada, err)
			}
		})
	}
}

func TestParse_FormaPagamento(t *testing.T) {
	casos := []struct {
		entrada string
		quero   parser.FormaPagamento
	}{
		{"mercado 120 pix", parser.Pix},
		{"mercado 120 dinheiro", parser.Dinheiro},
		{"mercado 120 débito", parser.Debito},
		{"mercado 120 debito", parser.Debito},
		{"mercado 120 crédito", parser.Credito},
		// "cartao" sozinho e ambiguo; credito e a leitura majoritaria, mesma
		// logica de Outros -> Despesa: escolher em vez de perguntar.
		{"mercado 120 cartão", parser.Credito},
		// Sem mencao, o parser nao inventa: quem aplica o padrao do usuario e
		// a camada que conhece configuracao de usuario, nao o dominio puro.
		{"mercado 120", parser.FormaNaoInformada},
	}

	for _, c := range casos {
		t.Run(c.entrada, func(t *testing.T) {
			got, err := parser.Parse(c.entrada, agora)
			if err != nil {
				t.Fatalf("Parse(%q) retornou erro inesperado: %v", c.entrada, err)
			}
			if got.Forma != c.quero {
				t.Errorf("Forma = %q, quero %q", got.Forma, c.quero)
			}
		})
	}
}

// A forma de pagamento nao pode sequestrar a categoria nem o valor.
func TestParse_FormaNaoAtrapalhaOResto(t *testing.T) {
	got, err := parser.Parse("mercado 120 no débito", agora)
	if err != nil {
		t.Fatalf("Parse retornou erro inesperado: %v", err)
	}
	if got.Centavos != 12000 {
		t.Errorf("Centavos = %d, quero 12000", got.Centavos)
	}
	if got.Categoria != parser.Mercado {
		t.Errorf("Categoria = %q, quero %q", got.Categoria, parser.Mercado)
	}
	if got.Forma != parser.Debito {
		t.Errorf("Forma = %q, quero %q", got.Forma, parser.Debito)
	}
}

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
			_, err := parser.Parse(c.entrada, agora)
			if !errors.Is(err, c.quero) {
				t.Errorf("Parse(%.30q) erro = %v, quero %v", c.entrada, err, c.quero)
			}
		})
	}
}

func TestParse_LimiteDeTamanho(t *testing.T) {
	// Exatamente no limite tem que passar; um caractere alem, nao.
	noLimite := strings.Repeat("a", parser.MaxEntrada-3) + " 10"
	if _, err := parser.Parse(noLimite, agora); err != nil {
		t.Errorf("entrada de %d chars (limite %d) devia passar, deu %v",
			len(noLimite), parser.MaxEntrada, err)
	}

	alem := strings.Repeat("a", parser.MaxEntrada) + " 10"
	if _, err := parser.Parse(alem, agora); !errors.Is(err, parser.ErrEntradaLonga) {
		t.Errorf("entrada de %d chars devia dar ErrEntradaLonga, deu %v", len(alem), err)
	}
}

func TestParse_SinalNegativoEIgnorado(t *testing.T) {
	// O sinal e redundante: a direcao do dinheiro ja esta em Tipo. Entao
	// "-5 mercado" e uma despesa de R$ 5,00, nao um erro nem um valor negativo.
	got, err := parser.Parse("-5 mercado", agora)
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
			got, err := parser.Parse(c.entrada, agora)
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
			got, err := parser.Parse(c.entrada, agora)
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
			got, err := parser.Parse(c.entrada, agora)
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
			got, err := parser.Parse(c.entrada, agora)
			if err != nil {
				t.Fatalf("Parse(%q) retornou erro inesperado: %v", c.entrada, err)
			}
			if got.Categoria != c.quero {
				t.Errorf("Categoria = %q, quero %q", got.Categoria, c.quero)
			}
		})
	}
}
