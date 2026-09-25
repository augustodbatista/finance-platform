// Package parser turns the user's natural-language input ("120 mercado") into
// a structured entry.
//
// Pure domain: no I/O, no database, no network. The product thesis is that
// friction kills habit, so this package is the shortest path between what the
// user types and a saved entry.
package parser

import "time"

// Tipo tells money coming in from money going out.
type Tipo string

const (
	Despesa Tipo = "despesa"
	Receita Tipo = "receita"
)

// Lancamento is the structured result of one user input.
//
// Centavos is int64 on purpose: money in float64 silently corrupts balances
// (0.1 + 0.2 != 0.3). Formatting for display happens at the presentation
// edge, never here.
type Lancamento struct {
	Centavos  int64
	Categoria Categoria
	Tipo      Tipo
	// Data is the purchase date, not the date it was recorded: whoever logs a
	// purchase "ontem" (yesterday) wants it to count on the day it happened.
	Data time.Time
	// Forma is empty when the user did not say. See FormaNaoInformada.
	Forma FormaPagamento
	// Parcelas is 1 for a single payment. Centavos is still the TOTAL; splitting
	// into installments belongs to whoever knows the card's closing day, in
	// package fatura.
	Parcelas int
}

// Parse turns the user's input into a Lancamento.
//
// The clock comes in as a parameter instead of time.Now() being called in
// here. Otherwise the function would no longer be pure and "ontem" would give
// a different result depending on the day it ran -- tests included.
//
// The input is free text from the user, so the size ceiling is checked before
// any work: this is the domain's trust boundary.
//
// Order matters. Tokens that contain digits but are not money are removed from
// the string BEFORE the amount is extracted; otherwise the "last number wins"
// rule would pick the wrong piece: "mercado 120 15/03" would become R$ 0,03.
func Parse(entrada string, agora time.Time) (Lancamento, error) {
	if len(entrada) > MaxEntrada {
		return Lancamento{}, ErrEntradaLonga
	}

	// Normalize once, up front: from here on everything works on the same
	// string, with no case or accents getting in the way.
	data, resto, err := extrairData(normalizar(entrada), agora)
	if err != nil {
		return Lancamento{}, err
	}

	parcelas, resto, err := extrairParcelas(resto)
	if err != nil {
		return Lancamento{}, err
	}

	centavos, err := extrairCentavos(resto)
	if err != nil {
		return Lancamento{}, err
	}

	categoria := classificar(resto)

	// Inference fills a gap, it does not override the user: only credit cards
	// allow installments, but whoever typed another method deserves to be
	// corrected on screen, not silently contradicted here.
	forma := formaDe(resto)
	if parcelas > 1 && forma == FormaNaoInformada {
		forma = Credito
	}

	return Lancamento{
		Centavos:  centavos,
		Categoria: categoria,
		Tipo:      tipoDe(categoria),
		Data:      data,
		Forma:     forma,
		Parcelas:  parcelas,
	}, nil
}
