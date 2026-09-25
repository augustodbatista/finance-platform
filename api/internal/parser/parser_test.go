package parser_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/fatura"
	"github.com/augustodbatista/finance-platform/api/internal/parser"
)

// agora is the tests' fixed clock. Parse takes the clock instead of calling
// time.Now() internally: otherwise "ontem" (yesterday) would change value
// depending on the day the test ran, and a test that depends on the calendar
// is not a test.
var agora = time.Date(2026, time.July, 30, 14, 30, 0, 0, time.UTC)

func TestParse_Data(t *testing.T) {
	casos := []struct {
		nome    string
		entrada string
		relogio time.Time
		quero   time.Time
	}{
		{"no mention means today", "mercado 120", agora,
			time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)},
		{"explicit today", "mercado 120 hoje", agora,
			time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)},
		{"yesterday", "mercado 120 ontem", agora,
			time.Date(2026, time.July, 29, 0, 0, 0, 0, time.UTC)},
		{"yesterday in upper case", "mercado 120 ONTEM", agora,
			time.Date(2026, time.July, 29, 0, 0, 0, 0, time.UTC)},
		{"dd/mm", "mercado 120 15/03", agora,
			time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC)},
		{"dd/mm/aaaa", "mercado 120 15/03/2024", agora,
			time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC)},
		{"dd/mm/aa", "mercado 120 15/03/25", agora,
			time.Date(2025, time.March, 15, 0, 0, 0, 0, time.UTC)},
		// dd/mm without a year resolves to the most recent past occurrence:
		// logging late is common, logging in the future is not.
		{"dd/mm not yet reached this year", "mercado 120 20/12",
			time.Date(2026, time.January, 15, 9, 0, 0, 0, time.UTC),
			time.Date(2025, time.December, 20, 0, 0, 0, 0, time.UTC)},
		// Yesterday across a month boundary.
		{"yesterday on the 1st", "mercado 120 ontem",
			time.Date(2026, time.March, 1, 8, 0, 0, 0, time.UTC),
			time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC)},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, err := parser.Parse(c.entrada, c.relogio)
			if err != nil {
				t.Fatalf("Parse(%q) returned unexpected error: %v", c.entrada, err)
			}
			if !got.Data.Equal(c.quero) {
				t.Errorf("Data = %s, want %s",
					got.Data.Format(time.RFC3339), c.quero.Format(time.RFC3339))
			}
		})
	}
}

// The "last number wins" rule collides with dates: without removing the token
// before extracting the amount, "mercado 120 15/03" would become R$ 0,03. A
// silent error about money is the failure mode this project treats as
// unacceptable.
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
				t.Fatalf("Parse(%q) returned unexpected error: %v", c.entrada, err)
			}
			if got.Centavos != c.quero {
				t.Errorf("Centavos = %d, want %d", got.Centavos, c.quero)
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
				t.Errorf("Parse(%q) error = %v, want ErrDataInvalida", entrada, err)
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
		// "cartao" (card) alone is ambiguous; credit is the majority reading,
		// same logic as Outros -> Despesa: pick instead of asking.
		{"mercado 120 cartão", parser.Credito},
		// With no mention, the parser does not invent one: applying the user's
		// default belongs to the layer that knows user settings, not the pure
		// domain.
		{"mercado 120", parser.FormaNaoInformada},
	}

	for _, c := range casos {
		t.Run(c.entrada, func(t *testing.T) {
			got, err := parser.Parse(c.entrada, agora)
			if err != nil {
				t.Fatalf("Parse(%q) returned unexpected error: %v", c.entrada, err)
			}
			if got.Forma != c.quero {
				t.Errorf("Forma = %q, want %q", got.Forma, c.quero)
			}
		})
	}
}

// The payment method must not hijack the category or the amount.
func TestParse_FormaNaoAtrapalhaOResto(t *testing.T) {
	got, err := parser.Parse("mercado 120 no débito", agora)
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}
	if got.Centavos != 12000 {
		t.Errorf("Centavos = %d, want 12000", got.Centavos)
	}
	if got.Categoria != parser.Mercado {
		t.Errorf("Categoria = %q, want %q", got.Categoria, parser.Mercado)
	}
	if got.Forma != parser.Debito {
		t.Errorf("Forma = %q, want %q", got.Forma, parser.Debito)
	}
}

func TestParse_Parcelas(t *testing.T) {
	casos := []struct {
		entrada       string
		queroParcelas int
		queroCentavos int64
	}{
		// The typed number is the purchase TOTAL; splitting is the system's job.
		{"3x 300 mercado", 3, 30000},
		// The trap: without removing the token, "last number wins" would pick
		// the 3 from "3x" and the entry would become R$ 3,00.
		{"300 mercado 3x", 3, 30000},
		{"mercado 3x 300", 3, 30000},
		{"12x 1200 tv", 12, 120000},
		{"3 x 300 mercado", 3, 30000},
		// A single payment is one installment, not zero: that keeps sums working.
		{"mercado 120", 1, 12000},
		{"1x 100 mercado", 1, 10000},
	}

	for _, c := range casos {
		t.Run(c.entrada, func(t *testing.T) {
			got, err := parser.Parse(c.entrada, agora)
			if err != nil {
				t.Fatalf("Parse(%q) returned unexpected error: %v", c.entrada, err)
			}
			if got.Parcelas != c.queroParcelas {
				t.Errorf("Parcelas = %d, want %d", got.Parcelas, c.queroParcelas)
			}
			if got.Centavos != c.queroCentavos {
				t.Errorf("Centavos = %d, want %d", got.Centavos, c.queroCentavos)
			}
		})
	}
}

func TestParse_ParcelamentoImplicaCredito(t *testing.T) {
	got, err := parser.Parse("3x 300 mercado", agora)
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}
	if got.Forma != parser.Credito {
		t.Errorf("Forma = %q, want %q: only credit cards allow installments", got.Forma, parser.Credito)
	}

	// Inference fills a gap, it does not override the user. Debit installments
	// do not exist, but whoever typed "debito" deserves to be respected and
	// corrected on screen -- not silently contradicted by the parser.
	got, err = parser.Parse("3x 300 mercado debito", agora)
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}
	if got.Forma != parser.Debito {
		t.Errorf("Forma = %q, want %q", got.Forma, parser.Debito)
	}
}

func TestParse_ParcelasInvalidas(t *testing.T) {
	casos := []string{
		"0x 100 mercado",
		"150x 100 mercado",
		"99999999999999999999999x 100 mercado",
	}

	for _, entrada := range casos {
		t.Run(entrada, func(t *testing.T) {
			_, err := parser.Parse(entrada, agora)
			if !errors.Is(err, fatura.ErrParcelasInvalidas) {
				t.Errorf("Parse(%.30q) error = %v, want ErrParcelasInvalidas", entrada, err)
			}
		})
	}
}

func TestParse_Erros(t *testing.T) {
	casos := []struct {
		nome    string
		entrada string
		quero   error
	}{
		{"description only", "mercado", parser.ErrSemValor},
		{"empty", "", parser.ErrSemValor},
		{"only spaces", "   ", parser.ErrSemValor},
		{"no digits at all", "almoço no centro", parser.ErrSemValor},
		// A zero entry means nothing and is probably a mistake.
		{"zero amount", "0 mercado", parser.ErrValorNaoPositivo},
		{"zero with decimals", "0,00 mercado", parser.ErrValorNaoPositivo},
		// 20 digits overflow int64 cents. It must fail here, not become a
		// silently corrupted balance later on.
		{"overflows int64", "99999999999999999999 mercado", parser.ErrValorInvalido},
		// Size limit: the parser receives free text from the user, and that is
		// a trust boundary.
		{"input too long", strings.Repeat("a", 201) + " 10", parser.ErrEntradaLonga},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, err := parser.Parse(c.entrada, agora)
			if !errors.Is(err, c.quero) {
				t.Errorf("Parse(%.30q) error = %v, want %v", c.entrada, err, c.quero)
			}
		})
	}
}

func TestParse_LimiteDeTamanho(t *testing.T) {
	// Exactly at the limit must pass; one character over must not.
	noLimite := strings.Repeat("a", parser.MaxEntrada-3) + " 10"
	if _, err := parser.Parse(noLimite, agora); err != nil {
		t.Errorf("input of %d chars (limit %d) should pass, got %v",
			len(noLimite), parser.MaxEntrada, err)
	}

	alem := strings.Repeat("a", parser.MaxEntrada) + " 10"
	if _, err := parser.Parse(alem, agora); !errors.Is(err, parser.ErrEntradaLonga) {
		t.Errorf("input of %d chars should give ErrEntradaLonga, got %v", len(alem), err)
	}
}

func TestParse_SinalNegativoEIgnorado(t *testing.T) {
	// The sign is redundant: the direction of money already lives in Tipo. So
	// "-5 mercado" is an expense of R$ 5,00, not an error nor a negative amount.
	got, err := parser.Parse("-5 mercado", agora)
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}
	if got.Centavos != 500 {
		t.Errorf("Centavos = %d, want 500", got.Centavos)
	}
	if got.Tipo != parser.Despesa {
		t.Errorf("Tipo = %q, want %q", got.Tipo, parser.Despesa)
	}
}

func TestParse_Valor(t *testing.T) {
	casos := []struct {
		entrada string
		quero   int64
	}{
		{"120 mercado", 12000},
		// The comma is the decimal separator in pt-BR.
		{"Almoço 42,50", 4250},
		// One decimal digit is still cents: 42,5 = 42 reais and 50 cents.
		{"cafe 42,5", 4250},
		// More than two decimals is a typo: truncate, do not round, so as not to
		// invent a cent the user did not type.
		{"cafe 42,555", 4255},
		// The dot is the thousands separator, and both can appear together.
		{"R$ 1.234,56", 123456},
		{"1.234", 123400},
		// More than one number: the last one wins (product decision, 2026-07-30).
		{"2 cafés 15", 1500},
		{"3x uber 18", 1800},
		// Amount before or after the description gives the same result.
		{"Uber 18", 1800},
		{"18 uber", 1800},
	}

	for _, c := range casos {
		t.Run(c.entrada, func(t *testing.T) {
			got, err := parser.Parse(c.entrada, agora)
			if err != nil {
				t.Fatalf("Parse(%q) returned unexpected error: %v", c.entrada, err)
			}
			if got.Centavos != c.quero {
				t.Errorf("Centavos = %d, want %d", got.Centavos, c.quero)
			}
		})
	}
}

func TestParse_Categoria(t *testing.T) {
	casos := []struct {
		entrada string
		quero   parser.Categoria
	}{
		// The category's own name is a valid term.
		{"120 mercado", parser.Mercado},
		// A brand name maps to its category.
		{"59 netflix", parser.Assinaturas},
		{"Uber 18", parser.Transporte},
		// Case and accents must not change the result.
		{"ALMOÇO 10", parser.Alimentacao},
		{"almoco 10", parser.Alimentacao},
		{"Almoço 10", parser.Alimentacao},
		// An unknown term falls into Outros: zero friction is worth more than
		// category precision, so this is not an error.
		{"xyzabc 30", parser.Outros},
	}

	for _, c := range casos {
		t.Run(c.entrada, func(t *testing.T) {
			got, err := parser.Parse(c.entrada, agora)
			if err != nil {
				t.Fatalf("Parse(%q) returned unexpected error: %v", c.entrada, err)
			}
			if got.Categoria != c.quero {
				t.Errorf("Categoria = %q, want %q", got.Categoria, c.quero)
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
		// An income category defines the type; the user does not need to say.
		{"Salário 3500", parser.Receita},
		{"freela 800", parser.Receita},
		{"dividendo 120", parser.Receita},
		// Outros is ambiguous by nature: with no sign of income it is an
		// expense, which is the overwhelming majority of entries.
		{"xyzabc 30", parser.Despesa},
	}

	for _, c := range casos {
		t.Run(c.entrada, func(t *testing.T) {
			got, err := parser.Parse(c.entrada, agora)
			if err != nil {
				t.Fatalf("Parse(%q) returned unexpected error: %v", c.entrada, err)
			}
			if got.Tipo != c.quero {
				t.Errorf("Tipo = %q, want %q", got.Tipo, c.quero)
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
				t.Fatalf("Parse(%q) returned unexpected error: %v", c.entrada, err)
			}
			if got.Categoria != c.quero {
				t.Errorf("Categoria = %q, want %q", got.Categoria, c.quero)
			}
		})
	}
}
