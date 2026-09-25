// Package fatura answers which credit card statement ("fatura") a purchase
// lands on, and splits an installment purchase across statements.
//
// Pure domain: no I/O, no database, no network. It works only with primitives
// and does not import package parser -- calendar math and splitting money are
// not text analysis, and merging the two would make one of the package names
// lie.
//
// The closing day is a parameter rather than a field of a Card entity because
// cards do not exist yet: without persistence there would be nowhere to store
// them. The entity arrives together with the database.
package fatura

import (
	"errors"
	"time"
)

// ErrDiaFechamentoInvalido reports a closing day outside 1..31.
var ErrDiaFechamentoInvalido = errors.New("fatura: closing day must be between 1 and 31")

// Competencia identifies a statement: year and month, no day.
//
// No day on purpose. A statement is a monthly bucket, and adding months to a
// (year, month) pair avoids the "January 31 + 1 month" problem entirely: there
// is no February 31 to overflow into.
type Competencia struct {
	Ano int
	Mes time.Month
}

// AdicionarMeses returns the statement n months ahead.
func (c Competencia) AdicionarMeses(n int) Competencia {
	// time.Date normalizes months outside 1..12, so December + 1 becomes
	// January of the next year with no modulo arithmetic here.
	t := time.Date(c.Ano, c.Mes+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
	return Competencia{Ano: t.Year(), Mes: t.Month()}
}

// De returns which statement a purchase lands on, given the card's closing day.
//
// A purchase made on the closing day itself goes to the next statement: the
// statement closes at the start of the day, so that day already belongs to the
// new cycle. (Product decision, 2026-07-30 -- this varies by card issuer.)
//
// The comparison is by day number, without clamping the closing day to the
// length of the month. That alone handles cards that close on the 31st: in
// February every day is below 31, so every purchase of the month lands on that
// month's statement, which is the correct behavior.
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
