package web

import (
	"errors"
	"net/http"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/armazem"
	"github.com/augustodbatista/finance-platform/api/internal/fatura"
	"github.com/augustodbatista/finance-platform/api/internal/resumo"
)

type faturaJSON struct {
	Competencia string         `json:"competencia"`
	Centavos    int64          `json:"centavos"`
	Pagamento   *pagamentoJSON `json:"pagamento"`
}

func paraFaturaJSON(f resumo.Fatura) faturaJSON {
	out := faturaJSON{Competencia: f.Competencia.String(), Centavos: f.Centavos}
	if f.Pagamento != nil {
		out.Pagamento = &pagamentoJSON{f.Pagamento.Data.Format(time.DateOnly), f.Pagamento.Centavos}
	}
	return out
}

// faturas lists the credit card statements: GET /api/faturas.
func (s *servidor) faturas(w http.ResponseWriter, _ *http.Request) {
	fs, err := resumo.Faturas(s.lancamentos(), s.Armazem.FaturasPagas(), s.DiaFechamento)
	if err != nil {
		falhar(w, http.StatusInternalServerError, "Não consegui calcular as faturas.")
		return
	}
	out := make([]faturaJSON, 0, len(fs)) // [] rather than null: the front end iterates it
	for _, f := range fs {
		out = append(out, paraFaturaJSON(f))
	}
	responder(w, http.StatusOK, out)
}

// pagarFatura records a statement payment: POST
// /api/faturas/{competencia}/pagamento with {"data": "YYYY-MM-DD", "valor":
// "1.234,56"}. Paying again corrects the payment.
//
// Only a statement that exists can be paid: a typo in the month must not create
// an expense out of nothing.
func (s *servidor) pagarFatura(w http.ResponseWriter, r *http.Request) {
	c, ok := competenciaDaRota(w, r)
	if !ok {
		return
	}
	data, centavos, ok := s.lerPagamento(w, r)
	if !ok {
		return
	}

	fs, err := resumo.Faturas(s.lancamentos(), s.Armazem.FaturasPagas(), s.DiaFechamento)
	if err != nil {
		falhar(w, http.StatusInternalServerError, "Não consegui calcular as faturas. Nada foi alterado.")
		return
	}
	i := indiceFatura(fs, c)
	if i < 0 {
		falhar(w, http.StatusNotFound, "Não há fatura nesse mês.")
		return
	}

	p, err := fatura.Pagar(c, data, centavos, s.Agora())
	if err != nil { // only ErrPagamentoNoFuturo: lerPagamento already refused amounts <= 0
		falhar(w, http.StatusBadRequest, mensagem(err))
		return
	}
	if err := s.Armazem.PagarFatura(p); err != nil {
		falhar(w, http.StatusInternalServerError, "Não consegui salvar. Nada foi alterado.")
		return
	}
	fs[i].Pagamento = &p
	responder(w, http.StatusOK, paraFaturaJSON(fs[i]))
}

// desfazerPagamentoFatura marks a statement as unpaid again:
// DELETE /api/faturas/{competencia}/pagamento.
func (s *servidor) desfazerPagamentoFatura(w http.ResponseWriter, r *http.Request) {
	c, ok := competenciaDaRota(w, r)
	if !ok {
		return
	}
	switch err := s.Armazem.DesfazerPagamentoFatura(c); {
	case errors.Is(err, armazem.ErrNaoEncontrado):
		falhar(w, http.StatusNotFound, "Essa fatura não está paga.")
	case err != nil:
		falhar(w, http.StatusInternalServerError, "Não consegui salvar. Nada foi alterado.")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func indiceFatura(fs []resumo.Fatura, c fatura.Competencia) int {
	for i, f := range fs {
		if f.Competencia == c {
			return i
		}
	}
	return -1
}

// competenciaDaRota reads {competencia} (YYYY-MM). On failure it answers 400
// and returns false.
func competenciaDaRota(w http.ResponseWriter, r *http.Request) (fatura.Competencia, bool) {
	c, err := fatura.ParseCompetencia(r.PathValue("competencia"))
	if err != nil {
		falhar(w, http.StatusBadRequest, "Mês da fatura inválido. Use AAAA-MM.")
		return fatura.Competencia{}, false
	}
	return c, true
}
