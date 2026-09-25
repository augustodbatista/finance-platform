// Package web exposes the domain over HTTP and serves the MVP page (ADR-0001).
//
// The page runs in the phone's browser, reaching the PC over the local network.
// That creates attack surface the pure domain did not have, handled here:
//
//   - Anyone on the same network: password via Basic Auth (the browser's
//     native prompt).
//   - CSRF: with Basic Auth the browser resends the password on its own, so
//     POST requires Content-Type application/json -- a form on another site
//     cannot send that without a CORS preflight, which this server never
//     grants.
//   - XSS: CSP default-src 'self' (nothing inline) and the front end uses
//     textContent.
//   - Oversized bodies: MaxBytesReader.
//
// User-facing messages are in Portuguese on purpose: the product and its users
// are Brazilian.
package web

import (
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/armazem"
	"github.com/augustodbatista/finance-platform/api/internal/fatura"
	"github.com/augustodbatista/finance-platform/api/internal/parser"
	"github.com/augustodbatista/finance-platform/api/internal/resumo"
)

//go:embed static
var static embed.FS

// maxCorpo caps request bodies. An entry is at most parser.MaxEntrada bytes;
// 4KB leaves plenty for the JSON envelope and escapes.
const maxCorpo = 4 << 10

// Config is what the server needs to run.
type Config struct {
	Armazem       *armazem.Armazem
	DiaFechamento int
	// An empty Senha (password) disables authentication. Whether that is
	// acceptable is decided by main, which only allows it on loopback.
	Senha string
	// Agora is the clock. Injected for the same reason as in the parser: a test
	// that depends on the calendar is not a test.
	Agora func() time.Time
}

// Novo builds the handler with every route and protection.
func Novo(c Config) http.Handler {
	s := &servidor{c}
	mux := http.NewServeMux()

	sub, _ := fs.Sub(static, "static") // "static" exists, or the embed would not compile
	mux.Handle("GET /", http.FileServerFS(sub))
	mux.HandleFunc("GET /api/lancamentos", s.listar)
	mux.HandleFunc("POST /api/lancamentos", s.lancar)
	mux.HandleFunc("DELETE /api/lancamentos/{id}", s.remover)
	mux.HandleFunc("GET /api/resumo", s.resumo)

	return cabecalhos(autenticar(c.Senha, mux))
}

type servidor struct{ Config }

type registroJSON struct {
	ID        int64  `json:"id"`
	Texto     string `json:"texto"`
	Centavos  int64  `json:"centavos"`
	Categoria string `json:"categoria"`
	Tipo      string `json:"tipo"`
	Data      string `json:"data"`
	Forma     string `json:"forma"`
	Parcelas  int    `json:"parcelas"`
}

func paraJSON(r armazem.Registro) registroJSON {
	l := r.Lancamento
	return registroJSON{
		ID: r.ID, Texto: r.Texto, Centavos: l.Centavos,
		Categoria: string(l.Categoria), Tipo: string(l.Tipo),
		Data: l.Data.Format(time.DateOnly), Forma: string(l.Forma), Parcelas: l.Parcelas,
	}
}

func (s *servidor) listar(w http.ResponseWriter, _ *http.Request) {
	rs := s.Armazem.Listar()
	out := make([]registroJSON, 0, len(rs)) // [] rather than null: the front end iterates it
	for _, r := range rs {
		out = append(out, paraJSON(r))
	}
	responder(w, http.StatusOK, out)
}

func (s *servidor) lancar(w http.ResponseWriter, r *http.Request) {
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
		falhar(w, http.StatusUnsupportedMediaType, "Envie application/json.")
		return
	}

	var corpo struct {
		Texto string `json:"texto"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCorpo)).Decode(&corpo); err != nil {
		var grande *http.MaxBytesError
		if errors.As(err, &grande) {
			falhar(w, http.StatusRequestEntityTooLarge, "Texto longo demais.")
			return
		}
		falhar(w, http.StatusBadRequest, "JSON inválido.")
		return
	}

	l, err := parser.Parse(corpo.Texto, s.Agora())
	if err != nil {
		falhar(w, http.StatusBadRequest, mensagem(err))
		return
	}

	reg, err := s.Armazem.Adicionar(corpo.Texto, l)
	if err != nil {
		falhar(w, http.StatusInternalServerError, "Não consegui salvar. Nada foi gravado.")
		return
	}
	responder(w, http.StatusCreated, paraJSON(reg))
}

func (s *servidor) remover(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		falhar(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	switch err := s.Armazem.Remover(id); {
	case errors.Is(err, armazem.ErrNaoEncontrado):
		falhar(w, http.StatusNotFound, "Lançamento não encontrado.")
	case err != nil:
		falhar(w, http.StatusInternalServerError, "Não consegui apagar. Nada foi alterado.")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *servidor) resumo(w http.ResponseWriter, r *http.Request) {
	ref := s.Agora()
	if m := r.URL.Query().Get("mes"); m != "" {
		t, err := time.Parse("2006-01", m)
		if err != nil {
			falhar(w, http.StatusBadRequest, "Mês inválido. Use AAAA-MM.")
			return
		}
		ref = t
	}
	mes := fatura.Competencia{Ano: ref.Year(), Mes: ref.Month()}

	rs := s.Armazem.Listar()
	ls := make([]parser.Lancamento, len(rs))
	for i, reg := range rs {
		ls[i] = reg.Lancamento
	}

	res, err := resumo.Mensal(mes, ls, s.DiaFechamento)
	if err != nil {
		falhar(w, http.StatusInternalServerError, "Não consegui calcular o resumo.")
		return
	}
	responder(w, http.StatusOK, map[string]any{
		"mes":      ref.Format("2006-01"),
		"receitas": res.ReceitasCentavos,
		"despesas": res.DespesasCentavos,
		"economia": res.EconomiaCentavos,
	})
}

// mensagem turns a domain error into something the user understands and can fix.
func mensagem(err error) string {
	switch {
	case errors.Is(err, parser.ErrSemValor):
		return "Não encontrei o valor. Exemplo: 120 mercado"
	case errors.Is(err, parser.ErrValorNaoPositivo):
		return "O valor precisa ser maior que zero."
	case errors.Is(err, parser.ErrValorInvalido):
		return "Valor grande demais."
	case errors.Is(err, parser.ErrDataInvalida):
		return "Essa data não existe."
	case errors.Is(err, fatura.ErrParcelasInvalidas):
		return "Número de parcelas inválido (de 1 a 99)."
	case errors.Is(err, parser.ErrEntradaLonga):
		return "Texto longo demais."
	default:
		return "Não entendi esse lançamento."
	}
}

func responder(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // headers already sent; no way to tell the client
}

func falhar(w http.ResponseWriter, status int, msg string) {
	responder(w, status, map[string]string{"erro": msg})
}

func cabecalhos(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// autenticar requires Basic Auth when a password is set. Both passwords are
// hashed with SHA-256 before the constant-time comparison: that way both sides
// have the same length and not even the password's length leaks through
// response timing.
func autenticar(senha string, next http.Handler) http.Handler {
	if senha == "" {
		return next
	}
	quero := sha256.Sum256([]byte(senha))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, dada, ok := r.BasicAuth()
		got := sha256.Sum256([]byte(dada))
		if !ok || subtle.ConstantTimeCompare(got[:], quero[:]) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="finance", charset="UTF-8"`)
			http.Error(w, "senha necessária", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
