package fatura

import (
	"cmp"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrPagamentoInvalido reports a statement payment of zero or less.
	ErrPagamentoInvalido = errors.New("fatura: paid amount must be greater than zero")
	// ErrPagamentoNoFuturo reports a statement payment dated after today.
	ErrPagamentoNoFuturo = errors.New("fatura: payment date is in the future")
)

// Pagamento records that a statement was paid: when, and how much actually left
// the pocket. The amount may differ from the sum of the installments (interest,
// purchases that were never logged, a discount); package resumo shows the
// difference as its own line instead of hiding it.
//
// Partial payment carried over to the next statement (rotativo) is not modeled:
// a payment closes its statement.
type Pagamento struct {
	Competencia Competencia `json:"competencia"`
	Data        time.Time   `json:"data"`
	Centavos    int64       `json:"centavos"`
}

// Pagar validates and builds the payment of statement c.
//
// A payment cannot be dated after today: "paid" means it already happened.
// Calendar days are compared, not instants -- the same rule as bills in
// package dominio; mixing time zones would refuse a payment dated today.
func Pagar(c Competencia, data time.Time, centavos int64, hoje time.Time) (Pagamento, error) {
	switch {
	case centavos <= 0:
		return Pagamento{}, ErrPagamentoInvalido
	case diaAntes(hoje, data):
		return Pagamento{}, ErrPagamentoNoFuturo
	}
	return Pagamento{Competencia: c, Data: data, Centavos: centavos}, nil
}

// diaAntes reports whether day a comes before day b on the calendar.
func diaAntes(a, b time.Time) bool {
	return cmp.Or(cmp.Compare(a.Year(), b.Year()),
		cmp.Compare(a.Month(), b.Month()),
		cmp.Compare(a.Day(), b.Day())) < 0
}

// String formats the statement month as "2026-09": the form used in the data
// file and in URLs.
func (c Competencia) String() string {
	return fmt.Sprintf("%04d-%02d", c.Ano, int(c.Mes))
}

// ParseCompetencia reads "2026-09".
func ParseCompetencia(s string) (Competencia, error) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return Competencia{}, fmt.Errorf("fatura: invalid statement month %q, want YYYY-MM", s)
	}
	return Competencia{Ano: t.Year(), Mes: t.Month()}, nil
}

// MarshalText and UnmarshalText make a Competencia read and write as "2026-09"
// in JSON, instead of {"Ano":2026,"Mes":9}.
func (c Competencia) MarshalText() ([]byte, error) { return []byte(c.String()), nil }

func (c *Competencia) UnmarshalText(b []byte) error {
	p, err := ParseCompetencia(string(b))
	if err != nil {
		return err
	}
	*c = p
	return nil
}
