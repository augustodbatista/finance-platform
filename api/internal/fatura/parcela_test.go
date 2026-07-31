package fatura_test

import (
	"errors"
	"testing"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/fatura"
)

func TestDividir_ValoresECompetencias(t *testing.T) {
	// R$ 300,00 em 3x, comprado em 05/07 com fechamento dia 28: divide exato e
	// cai em julho, agosto e setembro.
	got, err := fatura.Dividir(30000, 3, dia(2026, time.July, 5), 28)
	if err != nil {
		t.Fatalf("Dividir retornou erro inesperado: %v", err)
	}

	quero := []fatura.Parcela{
		{Numero: 1, Centavos: 10000, Competencia: fatura.Competencia{Ano: 2026, Mes: time.July}},
		{Numero: 2, Centavos: 10000, Competencia: fatura.Competencia{Ano: 2026, Mes: time.August}},
		{Numero: 3, Centavos: 10000, Competencia: fatura.Competencia{Ano: 2026, Mes: time.September}},
	}
	if len(got) != len(quero) {
		t.Fatalf("len = %d, quero %d", len(got), len(quero))
	}
	for i := range quero {
		if got[i] != quero[i] {
			t.Errorf("parcela %d = %+v, quero %+v", i+1, got[i], quero[i])
		}
	}
}

func TestDividir_SobraVaiNaPrimeira(t *testing.T) {
	// R$ 100,00 em 3x da 33,33 tres vezes e sobra 1 centavo. A convencao dos
	// emissores brasileiros poe a sobra na primeira parcela.
	got, err := fatura.Dividir(10000, 3, dia(2026, time.July, 5), 28)
	if err != nil {
		t.Fatalf("Dividir retornou erro inesperado: %v", err)
	}

	quero := []int64{3334, 3333, 3333}
	for i, c := range quero {
		if got[i].Centavos != c {
			t.Errorf("parcela %d = %d centavos, quero %d", i+1, got[i].Centavos, c)
		}
	}
}

// A invariante que importa: nenhum centavo pode evaporar nem aparecer do nada,
// para nenhum total e nenhum numero de parcelas.
func TestDividir_SomaSempreFechaComOTotal(t *testing.T) {
	totais := []int64{1, 2, 7, 99, 100, 101, 3333, 10000, 12345, 99999, 1_000_000_007}
	parcelas := []int{1, 2, 3, 4, 6, 7, 12, 13, 24, 99}

	for _, total := range totais {
		for _, n := range parcelas {
			if total < int64(n) {
				continue
			}
			ps, err := fatura.Dividir(total, n, dia(2026, time.July, 5), 28)
			if err != nil {
				t.Fatalf("Dividir(%d, %d) retornou erro inesperado: %v", total, n, err)
			}

			var soma int64
			for _, p := range ps {
				if p.Centavos <= 0 {
					t.Errorf("Dividir(%d, %d): parcela %d com %d centavos", total, n, p.Numero, p.Centavos)
				}
				soma += p.Centavos
			}
			if soma != total {
				t.Errorf("Dividir(%d, %d): soma das parcelas = %d", total, n, soma)
			}
		}
	}
}

func TestDividir_ViradaDeAno(t *testing.T) {
	// Compra em 15/12 com fechamento 28: primeira em dezembro, e as seguintes
	// atravessam para o ano novo.
	got, err := fatura.Dividir(30000, 3, dia(2026, time.December, 15), 28)
	if err != nil {
		t.Fatalf("Dividir retornou erro inesperado: %v", err)
	}

	quero := []fatura.Competencia{
		{Ano: 2026, Mes: time.December},
		{Ano: 2027, Mes: time.January},
		{Ano: 2027, Mes: time.February},
	}
	for i, c := range quero {
		if got[i].Competencia != c {
			t.Errorf("parcela %d na competencia %+v, quero %+v", i+1, got[i].Competencia, c)
		}
	}
}

func TestDividir_PrimeiraParcelaSegueARegraDoFechamento(t *testing.T) {
	// Compra no proprio dia do fechamento: a primeira parcela ja cai na fatura
	// seguinte, e as demais seguem a partir dali.
	got, err := fatura.Dividir(20000, 2, dia(2026, time.July, 28), 28)
	if err != nil {
		t.Fatalf("Dividir retornou erro inesperado: %v", err)
	}
	if got[0].Competencia.Mes != time.August {
		t.Errorf("primeira parcela em %s, quero August", got[0].Competencia.Mes)
	}
	if got[1].Competencia.Mes != time.September {
		t.Errorf("segunda parcela em %s, quero September", got[1].Competencia.Mes)
	}
}

func TestDividir_Erros(t *testing.T) {
	compra := dia(2026, time.July, 5)
	casos := []struct {
		nome     string
		total    int64
		parcelas int
		diaFecha int
		quero    error
	}{
		{"zero parcelas", 10000, 0, 28, fatura.ErrParcelasInvalidas},
		{"parcelas negativas", 10000, -3, 28, fatura.ErrParcelasInvalidas},
		{"acima do teto", 10000, fatura.MaxParcelas + 1, 28, fatura.ErrParcelasInvalidas},
		// 2 centavos em 3x daria uma parcela de zero, que nao descreve nada.
		{"total menor que o numero de parcelas", 2, 3, 28, fatura.ErrTotalInsuficiente},
		{"total zero", 0, 1, 28, fatura.ErrTotalInsuficiente},
		{"total negativo", -100, 1, 28, fatura.ErrTotalInsuficiente},
		{"dia de fechamento invalido", 10000, 3, 0, fatura.ErrDiaFechamentoInvalido},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, err := fatura.Dividir(c.total, c.parcelas, compra, c.diaFecha)
			if !errors.Is(err, c.quero) {
				t.Errorf("Dividir(%d, %d, _, %d) erro = %v, quero %v",
					c.total, c.parcelas, c.diaFecha, err, c.quero)
			}
		})
	}
}
