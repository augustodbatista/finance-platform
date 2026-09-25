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

var agora = time.Date(2026, time.September, 25, 10, 0, 0, 0, time.UTC)

func novo(t *testing.T, senha string) http.Handler {
	t.Helper()
	a, err := armazem.Abrir(filepath.Join(t.TempDir(), "dados.json"))
	if err != nil {
		t.Fatal(err)
	}
	return web.Novo(web.Config{
		Armazem:       a,
		DiaFechamento: 28,
		Senha:         senha,
		Agora:         func() time.Time { return agora },
	})
}

func req(t *testing.T, h http.Handler, metodo, alvo, corpo string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(metodo, alvo, strings.NewReader(corpo))
	if corpo != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func lancar(t *testing.T, h http.Handler, texto string) map[string]any {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"texto": texto})
	w := req(t, h, http.MethodPost, "/api/lancamentos", string(b))
	if w.Code != http.StatusCreated {
		t.Fatalf("POST %q: status %d, body %s", texto, w.Code, w.Body)
	}
	var r map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestLancarEListar(t *testing.T) {
	h := novo(t, "")
	r := lancar(t, h, "3x 300 mercado")
	if r["centavos"] != float64(30000) || r["parcelas"] != float64(3) ||
		r["forma"] != "credito" || r["texto"] != "3x 300 mercado" {
		t.Errorf("POST response = %v", r)
	}

	w := req(t, h, http.MethodGet, "/api/lancamentos", "")
	var lista []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &lista); err != nil {
		t.Fatalf("GET: %v (%s)", err, w.Body)
	}
	if len(lista) != 1 || lista[0]["texto"] != "3x 300 mercado" {
		t.Errorf("list = %v", lista)
	}
}

func TestListaVaziaEArrayNaoNull(t *testing.T) {
	w := req(t, novo(t, ""), http.MethodGet, "/api/lancamentos", "")
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Errorf("empty list = %q, want [] (the front end would iterate over null)", w.Body)
	}
}

// Input the parser rejects comes back as 400 with a readable message, not 500.
func TestLancar_EntradaInvalida(t *testing.T) {
	casos := map[string]string{
		`{"texto":"mercado"}`:   "valor",
		`{"texto":"0 mercado"}`: "maior que zero",
		`{"texto":"120 30/02"}`: "data",
		`{"texto":"150x 100"}`:  "parcelas",
		`{nao e json`:           "json",
		`{"texto":""}`:          "valor",
	}
	for corpo, trecho := range casos {
		t.Run(corpo, func(t *testing.T) {
			w := req(t, novo(t, ""), http.MethodPost, "/api/lancamentos", corpo)
			if w.Code != http.StatusBadRequest {
				t.Errorf("status %d, want 400", w.Code)
			}
			if !strings.Contains(strings.ToLower(w.Body.String()), trecho) {
				t.Errorf("message %q does not mention %q", w.Body, trecho)
			}
		})
	}
}

func TestLancar_CorpoGrandeERecusado(t *testing.T) {
	corpo := `{"texto":"` + strings.Repeat("a", 10_000) + `"}`
	w := req(t, novo(t, ""), http.MethodPost, "/api/lancamentos", corpo)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status %d, want 400 or 413", w.Code)
	}
}

// CSRF: a form on another site can only send form-urlencoded, multipart or
// text/plain without a preflight. Requiring JSON closes that door.
func TestLancar_ExigeContentTypeJSON(t *testing.T) {
	h := novo(t, "")
	r := httptest.NewRequest(http.MethodPost, "/api/lancamentos", strings.NewReader(`{"texto":"10 mercado"}`))
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status %d, want 415", w.Code)
	}
	if n := len(listar(t, h)); n != 0 {
		t.Errorf("saved %d entries with the wrong content type", n)
	}
}

func listar(t *testing.T, h http.Handler) []map[string]any {
	t.Helper()
	var l []map[string]any
	_ = json.Unmarshal(req(t, h, http.MethodGet, "/api/lancamentos", "").Body.Bytes(), &l)
	return l
}

func TestRemover(t *testing.T) {
	h := novo(t, "")
	id := int64(lancar(t, h, "10 mercado")["id"].(float64))
	alvo := "/api/lancamentos/" + strconv.FormatInt(id, 10)

	if w := req(t, h, http.MethodDelete, alvo, ""); w.Code != http.StatusNoContent {
		t.Errorf("DELETE: status %d, want 204", w.Code)
	}
	if w := req(t, h, http.MethodDelete, alvo, ""); w.Code != http.StatusNotFound {
		t.Errorf("DELETE again: status %d, want 404", w.Code)
	}
	if w := req(t, h, http.MethodDelete, "/api/lancamentos/abc", ""); w.Code != http.StatusBadRequest {
		t.Errorf("DELETE with invalid id: status %d, want 400", w.Code)
	}
}

func TestResumo(t *testing.T) {
	h := novo(t, "")
	lancar(t, h, "salario 3500")
	lancar(t, h, "120 mercado debito")
	lancar(t, h, "3x 300 casa") // credit inferred: R$ 100 in September

	var r map[string]any
	w := req(t, h, http.MethodGet, "/api/resumo?mes=2026-09", "")
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatalf("%v (%s)", err, w.Body)
	}
	if r["receitas"] != float64(350000) || r["despesas"] != float64(22000) || r["economia"] != float64(328000) {
		t.Errorf("September summary = %v", r)
	}

	// With no parameter, it is the current month.
	var r2 map[string]any
	_ = json.Unmarshal(req(t, h, http.MethodGet, "/api/resumo", "").Body.Bytes(), &r2)
	if r2["despesas"] != r["despesas"] {
		t.Errorf("summary with no month = %v, want September 2026", r2)
	}

	if w := req(t, h, http.MethodGet, "/api/resumo?mes=setembro", ""); w.Code != http.StatusBadRequest {
		t.Errorf("invalid month: status %d, want 400", w.Code)
	}
}

func TestSenha(t *testing.T) {
	h := novo(t, "segredo")

	if w := req(t, h, http.MethodGet, "/api/lancamentos", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("no password: status %d, want 401", w.Code)
	} else if !strings.HasPrefix(w.Header().Get("WWW-Authenticate"), "Basic") {
		t.Error("401 without WWW-Authenticate: the browser will not show the password prompt")
	}

	for _, senha := range []string{"errada", "segredo "} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.SetBasicAuth("qualquer", senha)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("password %q: status %d, want 401", senha, w.Code)
		}
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetBasicAuth("qualquer", "segredo")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("correct password: status %d, want 200", w.Code)
	}
}

func TestPaginaECabecalhosDeSeguranca(t *testing.T) {
	h := novo(t, "")
	w := req(t, h, http.MethodGet, "/", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<html") {
		t.Fatalf("GET /: status %d", w.Code)
	}
	if csp := w.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("CSP = %q", csp)
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options: nosniff")
	}

	if w := req(t, h, http.MethodGet, "/app.js", ""); w.Code != http.StatusOK {
		t.Errorf("GET /app.js: status %d", w.Code)
	}
}

func TestLancar_MensagensDosDemaisErros(t *testing.T) {
	casos := map[string]string{
		`{"texto":"99999999999999999999 mercado"}`:        "grande",
		`{"texto":"` + strings.Repeat("a", 250) + ` 10"}`: "longo",
	}
	for corpo, trecho := range casos {
		w := req(t, novo(t, ""), http.MethodPost, "/api/lancamentos", corpo)
		if w.Code != http.StatusBadRequest || !strings.Contains(strings.ToLower(w.Body.String()), trecho) {
			t.Errorf("status %d, body %s; want 400 mentioning %q", w.Code, w.Body, trecho)
		}
	}
}

// A disk failure becomes a 500 whose message says what did NOT happen -- the
// user needs to know the entry was not saved, so they type it again.
func TestFalhaDeGravacao(t *testing.T) {
	p := filepath.Join(t.TempDir(), "dados.json")
	a, err := armazem.Abrir(p)
	if err != nil {
		t.Fatal(err)
	}
	h := web.Novo(web.Config{Armazem: a, DiaFechamento: 28, Agora: func() time.Time { return agora }})
	id := int64(lancar(t, h, "10 mercado")["id"].(float64))

	// Replace the file with a directory: every later write fails at rename.
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}

	w := req(t, h, http.MethodPost, "/api/lancamentos", `{"texto":"20 uber"}`)
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "Nada foi gravado") {
		t.Errorf("POST: status %d, body %s", w.Code, w.Body)
	}
	w = req(t, h, http.MethodDelete, "/api/lancamentos/"+strconv.FormatInt(id, 10), "")
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "Nada foi alterado") {
		t.Errorf("DELETE: status %d, body %s", w.Code, w.Body)
	}
}

func TestResumo_ConfigInvalidaViraErro500(t *testing.T) {
	a, _ := armazem.Abrir(filepath.Join(t.TempDir(), "dados.json"))
	h := web.Novo(web.Config{Armazem: a, DiaFechamento: 0, Agora: func() time.Time { return agora }})
	lancar(t, h, "3x 300 tv") // credit: the only case that needs the closing day

	if w := req(t, h, http.MethodGet, "/api/resumo", ""); w.Code != http.StatusInternalServerError {
		t.Errorf("status %d, want 500", w.Code)
	}
}
