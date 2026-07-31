package parser

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// ErrParcelasInvalidas indica numero de parcelas zero, negativo ou acima do teto.
var ErrParcelasInvalidas = errors.New("parser: numero de parcelas invalido")

// MaxParcelas e o teto de parcelas aceitas.
//
// Nao e preciosismo: fatura.Dividir aloca um slice de tamanho N vindo direto da
// entrada do usuario, entao "999999999x 100" sem teto seria alocacao de bilhoes
// de itens a partir de uma linha de texto. Teto pequeno mata o vetor, e 99
// parcelas ja passa de qualquer plano que um emissor brasileiro oferece.
const MaxParcelas = 99

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
// Espera a entrada ja normalizada (ver normalizar).
func extrairParcelas(entrada string) (int, string, error) {
	m := reParcelas.FindStringSubmatch(entrada)
	if m == nil {
		return 1, entrada, nil
	}

	// Atoi tambem falha por estouro, e nao so por texto invalido: um numero de
	// 23 digitos cai aqui em vez de virar lixo silencioso.
	parcelas, err := strconv.Atoi(m[1])
	if err != nil || parcelas < 1 || parcelas > MaxParcelas {
		return 0, "", ErrParcelasInvalidas
	}

	return parcelas, strings.Replace(entrada, m[0], " ", 1), nil
}
