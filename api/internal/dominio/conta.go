package dominio

import (
	"errors"
	"time"
)

// Bills ("contas a pagar"): an expense logged before it is paid.
//
// An entry is in one of three states:
//
//   - regular: Vencimento zero. Paid on Data, for Centavos. Every entry logged
//     before bills existed is in this state, so old data keeps its meaning.
//   - bill to pay: Vencimento set, Pagamento zero.
//   - paid bill: Vencimento and Pagamento set. What counts as spent is the
//     payment -- its date and its amount -- not Data and Centavos, which stay
//     as the expected values so fines or discounts remain visible.
//
// Credit card purchases are never bills: they are paid through the statement.

var (
	ErrSemVencimento     = errors.New("dominio: a bill needs a due date")
	ErrContaNoCredito    = errors.New("dominio: credit card purchases are paid through the statement, not as bills")
	ErrContaReceita      = errors.New("dominio: income cannot be a bill to pay")
	ErrNaoEConta         = errors.New("dominio: entry is not a bill")
	ErrPagamentoInvalido = errors.New("dominio: paid amount must be greater than zero")
	ErrPagamentoNoFuturo = errors.New("dominio: payment date is in the future")
)

// Pagamento records when a bill was paid and how much actually left the pocket.
type Pagamento struct {
	Data     time.Time
	Centavos int64
}

// EConta reports whether the entry was logged as a bill to pay.
func (l Lancamento) EConta() bool { return !l.Vencimento.IsZero() }

// Pago reports whether a bill has been paid. Regular entries are not bills;
// for them the question does not apply and this returns false.
func (l Lancamento) Pago() bool { return !l.Pagamento.Data.IsZero() }

// ComoConta turns an expense into a bill due on vencimento.
func (l Lancamento) ComoConta(vencimento time.Time) (Lancamento, error) {
	switch {
	case vencimento.IsZero():
		return Lancamento{}, ErrSemVencimento
	case l.Tipo == Receita:
		return Lancamento{}, ErrContaReceita
	case l.Forma == Credito:
		return Lancamento{}, ErrContaNoCredito
	}
	l.Vencimento = vencimento
	return l, nil
}

// Pagar records the payment of a bill. Paying again replaces the previous
// payment, so a wrong date or amount can be corrected.
//
// hoje may carry a time of day; only its date matters. A payment cannot be
// dated after today: "paid" means it already happened.
func (l Lancamento) Pagar(data time.Time, centavos int64, hoje time.Time) (Lancamento, error) {
	diaDeHoje := time.Date(hoje.Year(), hoje.Month(), hoje.Day(), 0, 0, 0, 0, hoje.Location())
	switch {
	case !l.EConta():
		return Lancamento{}, ErrNaoEConta
	case centavos <= 0:
		return Lancamento{}, ErrPagamentoInvalido
	case data.After(diaDeHoje):
		return Lancamento{}, ErrPagamentoNoFuturo
	}
	l.Pagamento = Pagamento{Data: data, Centavos: centavos}
	return l, nil
}

// DesfazerPagamento turns a paid bill back into a bill to pay.
func (l Lancamento) DesfazerPagamento() Lancamento {
	l.Pagamento = Pagamento{}
	return l
}
