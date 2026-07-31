// Package fatura responde em qual fatura de cartao uma compra cai, e reparte
// uma compra parcelada entre faturas.
//
// E dominio puro: sem I/O, sem banco, sem rede. Trabalha so com primitivos e
// nao importa o pacote parser -- calendario e divisao de dinheiro nao sao
// analise de texto, e juntar as duas coisas faria o nome de um dos dois mentir.
//
// O dia de fechamento entra como parametro em vez de sair de uma entidade
// Cartao porque cartao ainda nao existe: sem persistencia, ele nao teria onde
// ser guardado. A entidade nasce junto com o banco.
package fatura

import (
	"errors"
	"time"
)

// ErrDiaFechamentoInvalido indica dia de fechamento fora de 1..31.
var ErrDiaFechamentoInvalido = errors.New("fatura: dia de fechamento deve estar entre 1 e 31")

// Competencia identifica uma fatura: ano e mes, sem dia.
//
// Sem dia de proposito. A fatura e um balde mensal, e somar meses num par
// (ano, mes) nao tem o problema de "31 de janeiro + 1 mes": nao existe 31 de
// fevereiro para estourar.
type Competencia struct {
	Ano int
	Mes time.Month
}

// AdicionarMeses devolve a competencia N meses adiante.
func (c Competencia) AdicionarMeses(n int) Competencia {
	// time.Date normaliza mes fora de 1..12, entao dezembro + 1 vira janeiro do
	// ano seguinte sem nenhuma conta de resto aqui.
	t := time.Date(c.Ano, c.Mes+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
	return Competencia{Ano: t.Year(), Mes: t.Month()}
}

// De devolve em qual fatura uma compra cai, dado o dia de fechamento do cartao.
//
// Compra feita no proprio dia do fechamento vai para a fatura seguinte: a
// fatura fecha no inicio do dia, entao o dia ja pertence ao ciclo novo.
// (Decisao do Augusto, 30/07/2026 -- varia por emissor.)
//
// A comparacao e por numero do dia, sem ajustar o dia de fechamento ao tamanho
// do mes. Isso resolve sozinho o cartao que fecha dia 31: em fevereiro todo dia
// e menor que 31, entao toda compra do mes cai na fatura do proprio mes, que e
// o comportamento certo.
func De(compra time.Time, diaFechamento int) (Competencia, error) {
	if diaFechamento < 1 || diaFechamento > 31 {
		return Competencia{}, ErrDiaFechamentoInvalido
	}

	c := Competencia{Ano: compra.Year(), Mes: compra.Month()}
	if compra.Day() >= diaFechamento {
		c = c.AdicionarMeses(1)
	}
	return c, nil
}
