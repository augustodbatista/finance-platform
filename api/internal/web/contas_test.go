package web_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/armazem"
	"github.com/augustodbatista/finance-platform/api/internal/web"
)

// Bills to pay over HTTP. agora (the tests' clock) is 2026-09-25.

func corpoJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("%v (%s)", err, w.Body)
	}
	return m
}

func lancarConta(t *testing.T, h http.Handler, texto, vencimento string) map[string]any {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"texto": texto, "vencimento": vencimento})
	w := req(t, h, http.MethodPost, "/api/lancamentos", string(b))
	if w.Code != http.StatusCreated {
		t.Fatalf("POST bill %q: status %d, body %s", texto, w.Code, w.Body)
	}
	return corpoJSON(t, w)
}

func pagamentoURL(r map[string]any) string {
	return "/api/lancamentos/" + strconv.FormatInt(int64(r["id"].(float64)), 10) + "/pagamento"
}

func pagar(t *testing.T, h http.Handler, r map[string]any, data, valor string) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"data": data, "valor": valor})
	return req(t, h, http.MethodPost, pagamentoURL(r), string(b))
}

func resumoDe(t *testing.T, h http.Handler, mes string) map[string]any {
	t.Helper()
	return corpoJSON(t, req(t, h, http.MethodGet, "/api/resumo?mes="+mes, ""))
}

func TestLancarConta(t *testing.T) {
	h := novo(t, "")
	r := lancarConta(t, h, "luz 180 pix", "2026-10-10")

	if r["vencimento"] != "2026-10-10" || r["situacao"] != "a_pagar" || r["pagamento"] != nil {
		t.Errorf("bill = %v; want due 2026-10-10, situacao a_pagar, no payment", r)
	}

	// A regular entry has none of it.
	comum := lancar(t, h, "10 mercado")
	if _, tem := comum["vencimento"]; tem || comum["situacao"] != "" {
		t.Errorf("regular entry = %v; want no vencimento and empty situacao", comum)
	}
}

func TestLancarConta_Recusas(t *testing.T) {
	casos := []struct {
		nome, texto, vencimento, trecho string
	}{
		{"credit purchase", "tv 300 credito", "2026-10-10", "fatura"},
		{"installments imply credit", "3x 300 tv", "2026-10-10", "fatura"},
		{"income", "salario 3500", "2026-10-10", "receita"},
		{"invalid date", "luz 180", "2026-13-01", "vencimento"},
		{"not a date", "luz 180", "amanha", "vencimento"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			h := novo(t, "")
			b, _ := json.Marshal(map[string]string{"texto": c.texto, "vencimento": c.vencimento})
			w := req(t, h, http.MethodPost, "/api/lancamentos", string(b))
			if w.Code != http.StatusBadRequest || !strings.Contains(strings.ToLower(w.Body.String()), c.trecho) {
				t.Errorf("status %d, body %s; want 400 mentioning %q", w.Code, w.Body, c.trecho)
			}
			if n := len(listar(t, h)); n != 0 {
				t.Errorf("a refused bill was saved (%d entries)", n)
			}
		})
	}
}

func TestPagarConta(t *testing.T) {
	h := novo(t, "")
	r := lancarConta(t, h, "luz 180 pix", "2026-09-20")

	w := pagar(t, h, r, "2026-09-25", "185,40")
	if w.Code != http.StatusOK {
		t.Fatalf("pay: status %d, body %s", w.Code, w.Body)
	}
	pago := corpoJSON(t, w)
	pg, _ := pago["pagamento"].(map[string]any)
	if pago["situacao"] != "paga" || pg["data"] != "2026-09-25" || pg["centavos"] != float64(18540) ||
		pago["centavos"] != float64(18000) {
		t.Errorf("paid bill = %v; want situacao paga, paid 18540 on 2026-09-25, expected 18000 kept", pago)
	}

	// The summary counts the paid amount, and nothing is pending any more.
	res := resumoDe(t, h, "2026-09")
	if res["despesas"] != float64(18540) || res["a_pagar"] != float64(0) {
		t.Errorf("September summary = %v; want despesas 18540, a_pagar 0", res)
	}
}

func TestPagarConta_Recusas(t *testing.T) {
	h := novo(t, "")
	conta := lancarConta(t, h, "luz 180 pix", "2026-09-20")
	comum := lancar(t, h, "10 mercado")

	casos := []struct {
		nome   string
		alvo   map[string]any
		data   string
		valor  string
		status int
		trecho string
	}{
		{"not a bill", comum, "2026-09-25", "10", http.StatusConflict, "não é uma conta"},
		{"date in the future", conta, "2026-09-26", "180", http.StatusBadRequest, "futuro"},
		{"invalid date", conta, "2026-02-30", "180", http.StatusBadRequest, "data"},
		{"no amount", conta, "2026-09-25", "abc", http.StatusBadRequest, "valor"},
		{"zero amount", conta, "2026-09-25", "0", http.StatusBadRequest, "maior que zero"},
		{"unknown entry", map[string]any{"id": float64(999)}, "2026-09-25", "180", http.StatusNotFound, "não encontrado"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			w := pagar(t, h, c.alvo, c.data, c.valor)
			if w.Code != c.status || !strings.Contains(strings.ToLower(w.Body.String()), c.trecho) {
				t.Errorf("status %d, body %s; want %d mentioning %q", w.Code, w.Body, c.status, c.trecho)
			}
		})
	}

	if res := resumoDe(t, h, "2026-09"); res["a_pagar"] != float64(18000) {
		t.Errorf("after refused payments, a_pagar = %v; want 18000 (bill still pending)", res["a_pagar"])
	}
}

// CSRF: the new write endpoint gets the same protection as the others.
func TestPagarConta_ExigeJSON(t *testing.T) {
	h := novo(t, "")
	conta := lancarConta(t, h, "luz 180 pix", "2026-09-20")

	r := httptest.NewRequest(http.MethodPost, pagamentoURL(conta), strings.NewReader(`{"data":"2026-09-25","valor":"180"}`))
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status %d, want 415", w.Code)
	}
}

func TestDesfazerPagamento(t *testing.T) {
	h := novo(t, "")
	conta := lancarConta(t, h, "luz 180 pix", "2026-09-30")
	pagar(t, h, conta, "2026-09-25", "180")

	w := req(t, h, http.MethodDelete, pagamentoURL(conta), "")
	if w.Code != http.StatusOK {
		t.Fatalf("undo: status %d, body %s", w.Code, w.Body)
	}
	if r := corpoJSON(t, w); r["situacao"] != "a_pagar" || r["pagamento"] != nil {
		t.Errorf("after undo = %v; want situacao a_pagar and no payment", r)
	}

	comum := lancar(t, h, "10 mercado")
	if w := req(t, h, http.MethodDelete, pagamentoURL(comum), ""); w.Code != http.StatusConflict {
		t.Errorf("undo on a regular entry: status %d, want 409", w.Code)
	}
}

// Overdue bills show up whatever month is on screen.
func TestContaVencida(t *testing.T) {
	h := novo(t, "")
	lancarConta(t, h, "luz 180 pix", "2026-09-20")      // overdue on 2026-09-25
	lancarConta(t, h, "internet 100 pix", "2026-09-25") // due today: not overdue

	situacoes := map[string]string{}
	for _, l := range listar(t, h) {
		situacoes[l["texto"].(string)] = l["situacao"].(string)
	}
	if situacoes["luz 180 pix"] != "vencida" || situacoes["internet 100 pix"] != "a_pagar" {
		t.Errorf("situacoes = %v; want luz vencida, internet a_pagar", situacoes)
	}

	venc, _ := resumoDe(t, h, "2026-11")["vencidas"].(map[string]any)
	if venc["quantidade"] != float64(1) || venc["centavos"] != float64(18000) {
		t.Errorf("vencidas in November's summary = %v; want 1 bill, 18000", venc)
	}
}

func TestPagarConta_IDInvalido(t *testing.T) {
	h := novo(t, "")
	if w := req(t, h, http.MethodPost, "/api/lancamentos/abc/pagamento", `{"data":"2026-09-25","valor":"10"}`); w.Code != http.StatusBadRequest {
		t.Errorf("pay with invalid id: status %d, want 400", w.Code)
	}
	if w := req(t, h, http.MethodDelete, "/api/lancamentos/abc/pagamento", ""); w.Code != http.StatusBadRequest {
		t.Errorf("undo with invalid id: status %d, want 400", w.Code)
	}
}

// A disk failure while paying must say what did NOT happen, so the user knows
// the bill is still marked as unpaid.
func TestPagarConta_FalhaDeGravacao(t *testing.T) {
	p := filepath.Join(t.TempDir(), "dados.json")
	a, err := armazem.Abrir(p)
	if err != nil {
		t.Fatal(err)
	}
	h := web.Novo(web.Config{Armazem: a, DiaFechamento: 28, Agora: func() time.Time { return agora }})
	conta := lancarConta(t, h, "luz 180 pix", "2026-09-20")

	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}

	w := pagar(t, h, conta, "2026-09-25", "180")
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "Nada foi alterado") {
		t.Errorf("status %d, body %s; want 500 saying nothing changed", w.Code, w.Body)
	}
}
