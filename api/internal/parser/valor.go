package parser

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var (
	// ErrSemValor indica entrada sem nenhum numero reconhecivel.
	ErrSemValor = errors.New("parser: entrada sem valor monetario")
	// ErrValorInvalido indica numero presente mas fora da faixa representavel.
	ErrValorInvalido = errors.New("parser: valor monetario invalido")
	// ErrValorNaoPositivo indica valor zero, que nao descreve lancamento algum.
	ErrValorNaoPositivo = errors.New("parser: valor deve ser maior que zero")
	// ErrEntradaLonga indica entrada acima de MaxEntrada.
	ErrEntradaLonga = errors.New("parser: entrada longa demais")
)

// MaxEntrada limita o tamanho da entrada, em bytes.
//
// O parser recebe string livre do usuario, e isso e fronteira de confianca: sem
// teto, uma entrada de megabytes vira trabalho proporcional em CPU e memoria.
// O regexp do Go e RE2 e nao sofre ReDoS, mas linear sobre entrada gigante
// ainda e gigante. 200 bytes sobram para qualquer lancamento de uma linha.
const MaxEntrada = 200

// reValor casa um numero monetario em pt-BR: digitos, pontos de milhar e uma
// parte decimal opcional depois da virgula. Casa o token inteiro de uma vez
// ("1.234,56") em vez de pedacos, para nao fatiar numero malformado em dois.
//
// O regexp do Go e RE2: linear, sem backtracking. Entrada hostil nao vira
// ReDoS aqui, e trocar por uma lib com backtracking perderia essa garantia.
var reValor = regexp.MustCompile(`\d[\d.]*(?:,\d+)?`)

// extrairCentavos devolve o valor da entrada, em centavos.
//
// Regras de pt-BR: virgula e separador decimal, ponto e separador de milhar.
// Havendo mais de um numero, vale o ultimo -- "2 cafes 15" e R$ 15,00. E uma
// regra que nunca rejeita nem pergunta, porque perguntar custa o habito.
func extrairCentavos(entrada string) (int64, error) {
	numeros := reValor.FindAllString(entrada, -1)
	if len(numeros) == 0 {
		return 0, ErrSemValor
	}

	inteiro, decimais, _ := strings.Cut(numeros[len(numeros)-1], ",")

	// Pontos sao separador de milhar e nao carregam informacao: 1.234 e 1234.
	// Nao ha guarda para inteiro vazio: reValor exige digito inicial, entao a
	// parte antes da virgula sempre tem ao menos um digito que sobrevive a
	// remocao dos pontos. Guarda inalcancavel mente sobre o que pode acontecer.
	inteiro = strings.ReplaceAll(inteiro, ".", "")

	// Centavos tem exatamente duas casas: "5" vira "50", "555" vira "55".
	switch {
	case len(decimais) == 0:
		decimais = "00"
	case len(decimais) == 1:
		decimais += "0"
	case len(decimais) > 2:
		decimais = decimais[:2]
	}

	// Montar a string de centavos e converter de uma vez deixa o overflow por
	// conta do ParseInt, em vez de uma multiplicacao que estoura em silencio.
	centavos, err := strconv.ParseInt(inteiro+decimais, 10, 64)
	if err != nil {
		return 0, ErrValorInvalido
	}
	if centavos == 0 {
		return 0, ErrValorNaoPositivo
	}
	return centavos, nil
}
