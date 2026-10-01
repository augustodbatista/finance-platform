package web

import (
	"errors"
	"net/http"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/armazem"
	"github.com/augustodbatista/finance-platform/api/internal/dominio"
	"github.com/augustodbatista/finance-platform/api/internal/parser"
)

// pagar records the payment of a bill: POST /api/lancamentos/{id}/pagamento
// with {"data": "YYYY-MM-DD", "valor": "185,40"}. Paying again corrects the
// payment.
//
// The amount goes through parser.Valor, the same pt-BR reading the entry text
// went through, so "185,40" means the same thing in both places.
func (s *servidor) pagar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	var corpo struct {
		Data  string `json:"data"`
		Valor string `json:"valor"`
	}
	if !lerJSON(w, r, &corpo) {
		return
	}

	data, err := time.ParseInLocation(time.DateOnly, corpo.Data, s.Agora().Location())
	if err != nil {
		falhar(w, http.StatusBadRequest, "Data de pagamento inválida.")
		return
	}
	centavos, err := parser.Valor(corpo.Valor)
	if errors.Is(err, parser.ErrSemValor) {
		falhar(w, http.StatusBadRequest, "Informe o valor pago, por exemplo 185,40.")
		return
	}
	if err != nil {
		falhar(w, http.StatusBadRequest, mensagem(err))
		return
	}

	reg, err := s.Armazem.Alterar(id, func(l dominio.Lancamento) (dominio.Lancamento, error) {
		return l.Pagar(data, centavos, s.Agora())
	})
	s.responderAlteracao(w, reg, err)
}

// desfazerPagamento turns a paid bill back into a bill to pay:
// DELETE /api/lancamentos/{id}/pagamento.
func (s *servidor) desfazerPagamento(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	reg, err := s.Armazem.Alterar(id, func(l dominio.Lancamento) (dominio.Lancamento, error) {
		if !l.EConta() {
			return l, dominio.ErrNaoEConta
		}
		return l.DesfazerPagamento(), nil
	})
	s.responderAlteracao(w, reg, err)
}

// responderAlteracao maps the outcome of Armazem.Alterar to HTTP. Domain
// refusals are the user's to fix (400/409, with a message saying how); any
// other error is a storage failure, reported as what did NOT happen.
func (s *servidor) responderAlteracao(w http.ResponseWriter, reg armazem.Registro, err error) {
	switch {
	case err == nil:
		responder(w, http.StatusOK, s.paraJSON(reg))
	case errors.Is(err, armazem.ErrNaoEncontrado):
		falhar(w, http.StatusNotFound, "Lançamento não encontrado.")
	case errors.Is(err, dominio.ErrNaoEConta):
		falhar(w, http.StatusConflict, mensagem(err))
	// ErrPagamentoInvalido (amount <= 0) cannot reach this point: parser.Valor
	// already refuses zero and negative amounts with its own message.
	case errors.Is(err, dominio.ErrPagamentoNoFuturo):
		falhar(w, http.StatusBadRequest, mensagem(err))
	default:
		falhar(w, http.StatusInternalServerError, "Não consegui salvar. Nada foi alterado.")
	}
}
