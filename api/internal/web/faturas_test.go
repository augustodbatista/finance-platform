package web_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/armazem"
	"github.com/augustodbatista/finance-platform/api/internal/web"
)

// Credit card statements over HTTP. agora is 2026-09-25 and the closing day is
// 28, so a purchase today lands on September's statement.

func faturas(t *testing.T, h http.Handler) []map[string]any {
	t.Helper()
	w := req(t, h, http.MethodGet, "/api/faturas", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/faturas: status %d, body %s", w.Code, w.Body)
	}
	var out []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("%v (%s)", err, w.Body)
	}
	return out
}

func pagarFatura(t *testing.T, h http.Handler, competencia, data, valor string) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"data": data, "valor": valor})
	return req(t, h, http.MethodPost, "/api/faturas/"+competencia+"/pagamento", string(b))
}

func TestFaturas_Lista(t *testing.T) {
	h := novo(t, "")
	if w := req(t, h, http.MethodGet, "/api/faturas", ""); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Errorf("no statements: body %s, want [] (null would break the front end loop)", w.Body)
	}

	lancar(t, h, "3x 300 casa") // R$ 100 on September, October and November
	lancar(t, h, "50 cinema credito")

	got := faturas(t, h)
	if len(got) != 3 || got[0]["competencia"] != "2026-11" || got[2]["competencia"] != "2026-09" ||
		got[2]["centavos"] != float64(15000) || got[2]["pagamento"] != nil {
		t.Errorf("faturas = %v; want November..September, September 15000 unpaid", got)
	}
}

func TestPagarFatura(t *testing.T) {
	h := novo(t, "")
	lancar(t, h, "3x 300 casa")
	lancar(t, h, "120 mercado debito")

	if r := resumoDe(t, h, "2026-09"); r["despesas"] != float64(12000) || r["a_pagar"] != float64(10000) {
		t.Fatalf("before paying: %v; want despesas 12000 (debit only), a_pagar 10000 (statement)", r)
	}

	// Paid with R$ 5 of interest.
	w := pagarFatura(t, h, "2026-09", "2026-09-25", "105")
	if w.Code != http.StatusOK {
		t.Fatalf("pay: status %d, body %s", w.Code, w.Body)
	}
	if pg, _ := corpoJSON(t, w)["pagamento"].(map[string]any); pg["data"] != "2026-09-25" || pg["centavos"] != float64(10500) {
		t.Errorf("response = %s; want the statement with its payment", w.Body)
	}

	r := resumoDe(t, h, "2026-09")
	if r["despesas"] != float64(22500) || r["a_pagar"] != float64(0) {
		t.Errorf("after paying: %v; want despesas 22500 (120 + 105), a_pagar 0", r)
	}
	if !strings.Contains(req(t, h, http.MethodGet, "/api/resumo?mes=2026-09", "").Body.String(),
		`{"categoria":"ajuste_fatura","centavos":500}`) {
		t.Errorf("summary categories = %v; want an ajuste_fatura line of 500", r["categorias"])
	}

	// Undo: the statement is pending again.
	if w := req(t, h, http.MethodDelete, "/api/faturas/2026-09/pagamento", ""); w.Code != http.StatusNoContent {
		t.Fatalf("undo: status %d, body %s", w.Code, w.Body)
	}
	if r := resumoDe(t, h, "2026-09"); r["despesas"] != float64(12000) || r["a_pagar"] != float64(10000) {
		t.Errorf("after undo: %v; want despesas 12000, a_pagar 10000", r)
	}
	if w := req(t, h, http.MethodDelete, "/api/faturas/2026-09/pagamento", ""); w.Code != http.StatusNotFound {
		t.Errorf("undo twice: status %d, want 404", w.Code)
	}
}

func TestPagarFatura_Recusas(t *testing.T) {
	h := novo(t, "")
	lancar(t, h, "3x 300 casa")

	casos := []struct {
		nome, competencia, data, valor string
		status                         int
		trecho                         string
	}{
		{"statement with no purchases", "2026-03", "2026-09-25", "100", http.StatusNotFound, "fatura"},
		{"invalid statement month", "2026-13", "2026-09-25", "100", http.StatusBadRequest, "aaaa-mm"},
		{"date in the future", "2026-09", "2026-09-26", "100", http.StatusBadRequest, "futuro"},
		{"invalid date", "2026-09", "2026-02-30", "100", http.StatusBadRequest, "data"},
		{"no amount", "2026-09", "2026-09-25", "abc", http.StatusBadRequest, "valor"},
		{"zero amount", "2026-09", "2026-09-25", "0", http.StatusBadRequest, "maior que zero"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			w := pagarFatura(t, h, c.competencia, c.data, c.valor)
			if w.Code != c.status || !strings.Contains(strings.ToLower(w.Body.String()), c.trecho) {
				t.Errorf("status %d, body %s; want %d mentioning %q", w.Code, w.Body, c.status, c.trecho)
			}
		})
	}
	if w := req(t, h, http.MethodDelete, "/api/faturas/setembro/pagamento", ""); w.Code != http.StatusBadRequest {
		t.Errorf("undo with invalid month: status %d, want 400", w.Code)
	}
	for _, f := range faturas(t, h) {
		if f["pagamento"] != nil {
			t.Errorf("a refused payment was saved: %v", f)
		}
	}
}

// CSRF: same protection as every other write endpoint.
func TestPagarFatura_ExigeJSON(t *testing.T) {
	h := novo(t, "")
	lancar(t, h, "3x 300 casa")

	r := httptest.NewRequest(http.MethodPost, "/api/faturas/2026-09/pagamento", strings.NewReader(`{"data":"2026-09-25","valor":"100"}`))
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status %d, want 415", w.Code)
	}
}

func TestFaturas_ConfigInvalidaViraErro500(t *testing.T) {
	a, _ := armazem.Abrir(filepath.Join(t.TempDir(), "dados.json"))
	h := web.Novo(web.Config{Armazem: a, DiaFechamento: 0, Agora: func() time.Time { return agora }})
	lancar(t, h, "3x 300 tv")

	if w := req(t, h, http.MethodGet, "/api/faturas", ""); w.Code != http.StatusInternalServerError {
		t.Errorf("GET: status %d, want 500", w.Code)
	}
	if w := pagarFatura(t, h, "2026-09", "2026-09-25", "100"); w.Code != http.StatusInternalServerError {
		t.Errorf("POST: status %d, want 500", w.Code)
	}
}

// A disk failure must say what did NOT happen.
func TestPagarFatura_FalhaDeGravacao(t *testing.T) {
	p := filepath.Join(t.TempDir(), "dados.json")
	a, err := armazem.Abrir(p)
	if err != nil {
		t.Fatal(err)
	}
	h := web.Novo(web.Config{Armazem: a, DiaFechamento: 28, Agora: func() time.Time { return agora }})
	lancar(t, h, "3x 300 casa")
	if w := pagarFatura(t, h, "2026-09", "2026-09-25", "100"); w.Code != http.StatusOK {
		t.Fatalf("pay: status %d", w.Code)
	}

	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}

	if w := pagarFatura(t, h, "2026-09", "2026-09-25", "110"); w.Code != http.StatusInternalServerError ||
		!strings.Contains(w.Body.String(), "Nada foi alterado") {
		t.Errorf("POST: status %d, body %s; want 500 saying nothing changed", w.Code, w.Body)
	}
	if w := req(t, h, http.MethodDelete, "/api/faturas/2026-09/pagamento", ""); w.Code != http.StatusInternalServerError ||
		!strings.Contains(w.Body.String(), "Nada foi alterado") {
		t.Errorf("DELETE: status %d, body %s; want 500 saying nothing changed", w.Code, w.Body)
	}
}
