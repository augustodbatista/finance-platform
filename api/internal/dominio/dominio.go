// Package dominio holds the entry type and the closed sets every other package
// shares: categories, payment methods and income/expense.
//
// It used to live in package parser, so resumo, armazem and web imported the
// text-parsing package just for these types. The trigger recorded in CLAUDE.md
// for moving it ("a third consumer appears") had fired; this package is the
// move, with no behavior change.
//
// Pure domain: no I/O, no dependencies.
package dominio

import "time"

// Tipo tells money coming in from money going out.
type Tipo string

const (
	Despesa Tipo = "despesa"
	Receita Tipo = "receita"
)

// Lancamento is one entry: an expense or an income.
//
// Centavos is int64 on purpose: money in float64 silently corrupts balances
// (0.1 + 0.2 != 0.3). Formatting for display happens at the presentation
// edge, never here.
//
// The field names are the on-disk format of dados.json (serialized without
// JSON tags). Renaming one makes existing files load with that field zeroed;
// armazem's TestFormatoV1ContinuaLegivel fails first.
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

	// Vencimento and Pagamento turn an entry into a bill; see conta.go.
	//
	// omitzero keeps them out of dados.json while unused: entries that are not
	// bills are written exactly as before, and files written before bills
	// existed load with both fields zero, which means "regular entry".
	Vencimento time.Time `json:",omitzero"`
	Pagamento  Pagamento `json:",omitzero"`
}

// Categoria is a closed set, not free text. Unrecognized input falls into
// Outros (other) and never becomes an error: the product thesis is that
// friction kills habit, and asking "which category?" costs more than
// classifying wrong.
type Categoria string

const (
	// Expenses.
	Alimentacao Categoria = "alimentacao"
	Mercado     Categoria = "mercado"
	Transporte  Categoria = "transporte"
	Casa        Categoria = "casa"
	Saude       Categoria = "saude"
	Lazer       Categoria = "lazer"
	Educacao    Categoria = "educacao"
	Assinaturas Categoria = "assinaturas"

	// Income.
	Salario       Categoria = "salario"
	Freelancer    Categoria = "freelancer"
	Investimentos Categoria = "investimentos"

	// Valid for both income and expenses.
	Outros Categoria = "outros"
)

// receitas are the categories that represent money coming in.
var receitas = map[Categoria]bool{
	Salario:       true,
	Freelancer:    true,
	Investimentos: true,
}

// TipoDe derives income or expense from the category, so the user never has to
// declare it: typing "salario 3500" already says everything needed.
//
// Outros falls into Despesa (expense). It is ambiguous by definition (it exists
// in both lists), and the overwhelming majority of entries are money going out
// -- the default that is wrong the least.
func TipoDe(c Categoria) Tipo {
	if receitas[c] {
		return Receita
	}
	return Despesa
}

// FormaPagamento says how money left or came in.
//
// It matters more than it looks: a credit card purchase does not change the
// balance today, only when the statement is paid. Without this, the dashboard
// lies to anyone who uses a card, which is almost everyone.
type FormaPagamento string

const (
	// FormaNaoInformada is the zero value: the user did not say.
	//
	// The parser reports what it found and does not invent a default. Applying
	// the user's preference is the job of the layer that knows user settings --
	// the pure domain should not know people's preferences.
	FormaNaoInformada FormaPagamento = ""

	Dinheiro FormaPagamento = "dinheiro"
	Pix      FormaPagamento = "pix"
	Debito   FormaPagamento = "debito"
	Credito  FormaPagamento = "credito"
)
