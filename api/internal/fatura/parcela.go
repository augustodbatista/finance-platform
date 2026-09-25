package fatura

import (
	"errors"
	"time"
)

var (
	// ErrParcelasInvalidas reports an installment count below 1 or above
	// MaxParcelas.
	ErrParcelasInvalidas = errors.New("fatura: invalid number of installments")
	// ErrTotalInsuficiente reports a total that cannot give each installment
	// at least one cent.
	ErrTotalInsuficiente = errors.New("fatura: total too small for the number of installments")
)

// MaxParcelas is the installment ceiling for the whole domain.
//
// It lives here and not in the parser because how many installments a card
// accepts is a card rule, not a text rule -- and because this is where the risk
// materializes: Dividir allocates a slice of length N that ultimately comes from
// a line typed by the user. Without a ceiling, "999999999x 100" would allocate
// billions of items from one sentence. 99 already exceeds any plan a Brazilian
// card issuer offers.
const MaxParcelas = 99

// Parcela is one installment of a purchase, with the statement it lands on.
type Parcela struct {
	Numero      int
	Centavos    int64
	Competencia Competencia
}

// Dividir splits a purchase total into installments, each on its statement.
//
// The total is the full purchase amount: "3x 300" is three installments of
// R$ 100, not three of R$ 300. The user types what they spent and the system
// does the math.
//
// When the division is not exact, the remainder goes on the first installment
// -- the Brazilian issuers' convention, so the app matches the real statement.
// The installments always add up to the total: no cent ever disappears.
//
// The first installment lands on the statement of the purchase date (see De)
// and each following one moves one month ahead. Since Competencia has no day,
// moving months here never hits a short month.
func Dividir(totalCentavos int64, parcelas int, compra time.Time, diaFechamento int) ([]Parcela, error) {
	if parcelas < 1 || parcelas > MaxParcelas {
		return nil, ErrParcelasInvalidas
	}
	// One check covers a zero total, a negative total, and a total that would
	// produce a zero-cent installment -- none of the three describes a purchase.
	if totalCentavos < int64(parcelas) {
		return nil, ErrTotalInsuficiente
	}

	primeira, err := De(compra, diaFechamento)
	if err != nil {
		return nil, err
	}

	base := totalCentavos / int64(parcelas)
	sobra := totalCentavos % int64(parcelas)

	ps := make([]Parcela, parcelas)
	for i := range ps {
		centavos := base
		if i == 0 {
			centavos += sobra
		}
		ps[i] = Parcela{
			Numero:      i + 1,
			Centavos:    centavos,
			Competencia: primeira.AdicionarMeses(i),
		}
	}
	return ps, nil
}
