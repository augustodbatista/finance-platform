// Package parser converte a entrada em linguagem natural do usuario
// ("120 mercado") em um lancamento estruturado.
//
// E dominio puro: sem I/O, sem banco, sem rede. A tese do produto e que atrito
// mata habito, entao este pacote e o caminho mais curto entre o que o usuario
// digita e um lancamento salvo.
package parser

import (
	"errors"
	"regexp"
	"strconv"
)

// Tipo distingue entrada de saida de dinheiro.
type Tipo string

const (
	Despesa Tipo = "despesa"
	Receita Tipo = "receita"
)

// Lancamento e o resultado estruturado de uma entrada do usuario.
//
// Centavos e int64 de proposito: dinheiro em float64 corrompe saldo
// silenciosamente (0.1 + 0.2 != 0.3). A formatacao para exibicao acontece na
// borda de apresentacao, nunca aqui.
type Lancamento struct {
	Centavos  int64
	Categoria Categoria
	Tipo      Tipo
}

// ErrSemValor indica que a entrada nao contem um valor monetario reconhecivel.
var ErrSemValor = errors.New("parser: entrada sem valor monetario")

// reNumero casa uma sequencia de digitos. O regexp do Go e RE2, linear e sem
// backtracking, entao entrada hostil nao vira ReDoS.
var reNumero = regexp.MustCompile(`\d+`)

// Parse converte a entrada do usuario em um Lancamento.
func Parse(entrada string) (Lancamento, error) {
	numero := reNumero.FindString(entrada)
	if numero == "" {
		return Lancamento{}, ErrSemValor
	}

	reais, err := strconv.ParseInt(numero, 10, 64)
	if err != nil {
		return Lancamento{}, ErrSemValor
	}

	return Lancamento{
		Centavos:  reais * 100,
		Categoria: classificar(entrada),
		Tipo:      Despesa,
	}, nil
}
