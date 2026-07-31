package parser

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/augustodbatista/finance-platform/api/internal/fatura"
)

// reParcelas casa "3x", "3 x" e "12x1200".
//
// Sem \b no fim de proposito: com ele, "12x1200" nao casaria, porque entre o
// "x" e o "1" nao ha fronteira de palavra -- e o token ficaria na string para
// o extrator de valor confundir.
var reParcelas = regexp.MustCompile(`\b(\d+)\s*x`)

// extrairParcelas devolve o numero de parcelas e a entrada sem o token.
//
// Ausencia de token significa uma parcela, nao zero: assim somar parcelas
// funciona igual para compra a vista e parcelada, sem caso especial.
//
// Devolver a entrada limpa e correcao, nao conveniencia. A regra "vale o
// ultimo numero" faria "300 mercado 3x" virar R$ 3,00 se o token continuasse
// na string na hora de extrair o valor.
//
// O teto e o erro vem de fatura, e nao daqui: quantas parcelas um cartao
// aceita e regra de cartao, nao de texto. Duplicar o limite nos dois pacotes
// seria duas verdades sobre a mesma coisa, prontas para divergir.
//
// Espera a entrada ja normalizada (ver normalizar).
func extrairParcelas(entrada string) (int, string, error) {
	m := reParcelas.FindStringSubmatch(entrada)
	if m == nil {
		return 1, entrada, nil
	}

	// Atoi tambem falha por estouro, e nao so por texto invalido: um numero de
	// 23 digitos cai aqui em vez de virar lixo silencioso.
	parcelas, err := strconv.Atoi(m[1])
	if err != nil || parcelas < 1 || parcelas > fatura.MaxParcelas {
		return 0, "", fatura.ErrParcelasInvalidas
	}

	return parcelas, strings.Replace(entrada, m[0], " ", 1), nil
}
