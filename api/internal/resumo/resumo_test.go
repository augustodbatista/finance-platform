package resumo_test

import (
	"errors"
	"testing"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/fatura"
	"github.com/augustodbatista/finance-platform/api/internal/parser"
	"github.com/augustodbatista/finance-platform/api/internal/resumo"
)

const fechamento = 28

func dia(ano int, mes time.Month, d int) time.Time {
	return time.Date(ano, mes, d, 0, 0, 0, 0, time.UTC)
}

func comp(ano int, mes time.Month) fatura.Competencia {
	return fatura.Competencia{Ano: ano, Mes: mes}
}

// lancamentos monta o mesmo conjunto para todos os testes: um mes com receita,
// despesa a vista, compra parcelada no credito, um lancamento do mes anterior e
// uma compra feita no proprio dia do fechamento.
func lancamentos() []parser.Lancamento {
	return []parser.Lancamento{
		{Centavos: 350000, Categoria: parser.Salario, Tipo: parser.Receita,
			Data: dia(2026, time.July, 5), Forma: parser.Pix, Parcelas: 1},
		{Centavos: 12000, Categoria: parser.Mercado, Tipo: parser.Despesa,
			Data: dia(2026, time.July, 5), Forma: parser.Debito, Parcelas: 1},
		// R$ 300 em 3x no credito: R$ 100 em julho, agosto e setembro.
		{Centavos: 30000, Categoria: parser.Casa, Tipo: parser.Despesa,
			Data: dia(2026, time.July, 5), Forma: parser.Credito, Parcelas: 3},
		// Mes anterior: nao pode vazar para julho.
		{Centavos: 1800, Categoria: parser.Transporte, Tipo: parser.Despesa,
			Data: dia(2026, time.June, 10), Forma: parser.Debito, Parcelas: 1},
		// Credito no proprio dia do fechamento: cai em agosto, nao em julho.
		{Centavos: 20000, Categoria: parser.Lazer, Tipo: parser.Despesa,
			Data: dia(2026, time.July, 28), Forma: parser.Credito, Parcelas: 1},
	}
}

func TestMensal(t *testing.T) {
	casos := []struct {
		nome          string
		mes           fatura.Competencia
		queroReceitas int64
		queroDespesas int64
		queroEconomia int64
	}{
		// Julho: salario 3500; despesas = mercado 120 (debito, conta no dia) +
		// primeira parcela da compra parcelada (100). A compra do dia 28 nao
		// entra: a fatura dela e agosto.
		{"julho", comp(2026, time.July), 350000, 22000, 328000},
		// Agosto: so faturas. Parcela 2 da compra parcelada (100) + a compra
		// feita no dia do fechamento de julho (200).
		{"agosto", comp(2026, time.August), 0, 30000, -30000},
		// Setembro: so a ultima parcela.
		{"setembro", comp(2026, time.September), 0, 10000, -10000},
		// Junho: so o uber a debito.
		{"junho", comp(2026, time.June), 0, 1800, -1800},
		{"mes sem nada", comp(2026, time.November), 0, 0, 0},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, err := resumo.Mensal(c.mes, lancamentos(), fechamento)
			if err != nil {
				t.Fatalf("Mensal retornou erro inesperado: %v", err)
			}
			if got.ReceitasCentavos != c.queroReceitas {
				t.Errorf("Receitas = %d, quero %d", got.ReceitasCentavos, c.queroReceitas)
			}
			if got.DespesasCentavos != c.queroDespesas {
				t.Errorf("Despesas = %d, quero %d", got.DespesasCentavos, c.queroDespesas)
			}
			if got.EconomiaCentavos != c.queroEconomia {
				t.Errorf("Economia = %d, quero %d", got.EconomiaCentavos, c.queroEconomia)
			}
		})
	}
}

// A despesa a vista conta no dia; a no credito conta na fatura. Sao dois
// numeros diferentes, e confundi-los e o erro que este pacote existe para
// evitar.
func TestMensal_CreditoContaNaFaturaEDebitoNoDia(t *testing.T) {
	compra := dia(2026, time.July, 29) // depois do fechamento dia 28

	debito := []parser.Lancamento{{Centavos: 5000, Tipo: parser.Despesa,
		Data: compra, Forma: parser.Debito, Parcelas: 1}}
	credito := []parser.Lancamento{{Centavos: 5000, Tipo: parser.Despesa,
		Data: compra, Forma: parser.Credito, Parcelas: 1}}

	julho, agosto := comp(2026, time.July), comp(2026, time.August)

	if r, _ := resumo.Mensal(julho, debito, fechamento); r.DespesasCentavos != 5000 {
		t.Errorf("debito em julho = %d, quero 5000", r.DespesasCentavos)
	}
	if r, _ := resumo.Mensal(julho, credito, fechamento); r.DespesasCentavos != 0 {
		t.Errorf("credito em julho = %d, quero 0: a fatura e agosto", r.DespesasCentavos)
	}
	if r, _ := resumo.Mensal(agosto, credito, fechamento); r.DespesasCentavos != 5000 {
		t.Errorf("credito em agosto = %d, quero 5000", r.DespesasCentavos)
	}
}

// Receita nao passa por fatura: salario no dia 29 e receita de julho mesmo com
// fechamento dia 28. Cartao de credito e forma de gastar, nao de receber.
func TestMensal_ReceitaIgnoraFatura(t *testing.T) {
	ls := []parser.Lancamento{{Centavos: 350000, Tipo: parser.Receita,
		Data: dia(2026, time.July, 29), Forma: parser.Credito, Parcelas: 3}}

	got, err := resumo.Mensal(comp(2026, time.July), ls, fechamento)
	if err != nil {
		t.Fatalf("Mensal retornou erro inesperado: %v", err)
	}
	if got.ReceitasCentavos != 350000 {
		t.Errorf("Receitas = %d, quero 350000 (inteiras, no mes da data)", got.ReceitasCentavos)
	}
}

func TestMensal_PropagaErroDeFatura(t *testing.T) {
	ls := []parser.Lancamento{{Centavos: 30000, Tipo: parser.Despesa,
		Data: dia(2026, time.July, 5), Forma: parser.Credito, Parcelas: 3}}

	if _, err := resumo.Mensal(comp(2026, time.July), ls, 0); !errors.Is(err, fatura.ErrDiaFechamentoInvalido) {
		t.Errorf("erro = %v, quero ErrDiaFechamentoInvalido", err)
	}
}
