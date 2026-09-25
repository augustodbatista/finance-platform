package armazem_test

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/armazem"
	"github.com/augustodbatista/finance-platform/api/internal/parser"
)

var agora = time.Date(2026, time.September, 25, 10, 0, 0, 0, time.UTC)

func caminho(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "dados.json")
}

func lanc(t *testing.T, texto string) parser.Lancamento {
	t.Helper()
	l, err := parser.Parse(texto, agora)
	if err != nil {
		t.Fatalf("Parse(%q): %v", texto, err)
	}
	return l
}

func abrir(t *testing.T, p string) *armazem.Armazem {
	t.Helper()
	a, err := armazem.Abrir(p)
	if err != nil {
		t.Fatalf("Abrir: %v", err)
	}
	return a
}

func TestAbrir_ArquivoInexistenteComecaVazio(t *testing.T) {
	a := abrir(t, caminho(t))
	if n := len(a.Listar()); n != 0 {
		t.Errorf("Listar() has %d records, want 0", n)
	}
}

// The test that matters: what was saved survives closing and reopening, field
// by field. If this fails, the app loses recorded money.
func TestAdicionar_PersisteEntreAberturas(t *testing.T) {
	p := caminho(t)
	l := lanc(t, "3x 300 mercado credito 15/09")

	r, err := abrir(t, p).Adicionar("3x 300 mercado credito 15/09", l)
	if err != nil {
		t.Fatalf("Adicionar: %v", err)
	}

	lista := abrir(t, p).Listar()
	if len(lista) != 1 {
		t.Fatalf("after reopening, %d records, want 1", len(lista))
	}
	got := lista[0]
	if got.ID != r.ID || got.Texto != "3x 300 mercado credito 15/09" {
		t.Errorf("record = %+v, want ID %d and the original text", got, r.ID)
	}
	gl := got.Lancamento
	if gl.Centavos != l.Centavos || gl.Categoria != l.Categoria || gl.Tipo != l.Tipo ||
		gl.Forma != l.Forma || gl.Parcelas != l.Parcelas || !gl.Data.Equal(l.Data) {
		t.Errorf("reopened entry = %+v, want %+v", gl, l)
	}
}

func TestListar_MaisRecentePrimeiro(t *testing.T) {
	a := abrir(t, caminho(t))
	for _, txt := range []string{"10 mercado", "20 uber", "30 netflix"} {
		if _, err := a.Adicionar(txt, lanc(t, txt)); err != nil {
			t.Fatal(err)
		}
	}

	lista := a.Listar()
	if lista[0].Texto != "30 netflix" || lista[2].Texto != "10 mercado" {
		t.Errorf("order = %q, %q, %q; want newest first",
			lista[0].Texto, lista[1].Texto, lista[2].Texto)
	}
}

func TestListar_DevolveCopia(t *testing.T) {
	a := abrir(t, caminho(t))
	if _, err := a.Adicionar("10 mercado", lanc(t, "10 mercado")); err != nil {
		t.Fatal(err)
	}

	a.Listar()[0].Texto = "adulterado"
	if a.Listar()[0].Texto != "10 mercado" {
		t.Error("changing the returned slice altered internal state")
	}
}

func TestRemover(t *testing.T) {
	p := caminho(t)
	a := abrir(t, p)
	r1, _ := a.Adicionar("10 mercado", lanc(t, "10 mercado"))
	r2, _ := a.Adicionar("20 uber", lanc(t, "20 uber"))

	if err := a.Remover(r1.ID); err != nil {
		t.Fatalf("Remover: %v", err)
	}

	lista := abrir(t, p).Listar()
	if len(lista) != 1 || lista[0].ID != r2.ID {
		t.Errorf("after removing and reopening: %+v, want only ID %d", lista, r2.ID)
	}

	if err := a.Remover(r1.ID); !errors.Is(err, armazem.ErrNaoEncontrado) {
		t.Errorf("removing again: error = %v, want ErrNaoEncontrado", err)
	}
}

// A reused ID would let a delayed DELETE remove the wrong record.
func TestIDNuncaEReaproveitado(t *testing.T) {
	p := caminho(t)
	a := abrir(t, p)
	r1, _ := a.Adicionar("10 mercado", lanc(t, "10 mercado"))
	r2, _ := a.Adicionar("20 uber", lanc(t, "20 uber"))
	if err := a.Remover(r2.ID); err != nil {
		t.Fatal(err)
	}

	r3, _ := abrir(t, p).Adicionar("30 netflix", lanc(t, "30 netflix"))
	if r3.ID == r1.ID || r3.ID == r2.ID {
		t.Errorf("ID %d reused (existing/removed: %d, %d)", r3.ID, r1.ID, r2.ID)
	}
}

// An unreadable file must not become "start empty": the next Adicionar would
// overwrite the file and erase everything in it.
func TestAbrir_ArquivoCorrompidoFalhaEmVezDeZerar(t *testing.T) {
	p := caminho(t)
	if err := os.WriteFile(p, []byte("{nao e json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := armazem.Abrir(p); err == nil {
		t.Fatal("Abrir of a corrupted file should fail")
	}
	if b, _ := os.ReadFile(filepath.Clean(p)); string(b) != "{nao e json" {
		t.Error("Abrir changed the corrupted file; it should leave it untouched for recovery")
	}
}

func TestGravacao_NaoDeixaTemporarioENaoExpoeAOutros(t *testing.T) {
	p := caminho(t)
	a := abrir(t, p)
	if _, err := a.Adicionar("10 mercado", lanc(t, "10 mercado")); err != nil {
		t.Fatal(err)
	}

	entradas, _ := os.ReadDir(filepath.Dir(p))
	if len(entradas) != 1 {
		t.Errorf("directory has %d files, want only the data file (leftover temp file?)", len(entradas))
	}
}

// HTTP handlers run in parallel. Without a lock, concurrent writes lose
// entries -- and CI's -race flags the data race.
func TestAdicionar_Concorrente(t *testing.T) {
	p := caminho(t)
	a := abrir(t, p)

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := a.Adicionar("10 mercado", lanc(t, "10 mercado")); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if n := len(abrir(t, p).Listar()); n != 50 {
		t.Errorf("%d records persisted, want 50", n)
	}
}

func TestAbrir_ErroDeLeituraNaoViraVazio(t *testing.T) {
	// A directory where the file should be: it exists but cannot be read.
	if _, err := armazem.Abrir(t.TempDir()); err == nil {
		t.Fatal("Abrir of an unreadable path should fail, not start empty")
	}
}

// If the disk refuses the write, memory and disk must stay the same. Otherwise
// the screen shows an entry that will vanish on the next restart.
func TestAdicionar_FalhaDeGravacaoNaoAlteraMemoria(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nao-existe", "dados.json")
	a := abrir(t, p)

	if _, err := a.Adicionar("10 mercado", lanc(t, "10 mercado")); err == nil {
		t.Fatal("Adicionar into a missing directory should fail")
	}
	if n := len(a.Listar()); n != 0 {
		t.Errorf("memory has %d records after a failed write, want 0", n)
	}
}

func TestRemover_FalhaDeGravacaoNaoAlteraMemoria(t *testing.T) {
	p := caminho(t)
	a := abrir(t, p)
	r, _ := a.Adicionar("10 mercado", lanc(t, "10 mercado"))

	// Replace the file with a directory: the final rename now fails.
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := a.Remover(r.ID); err == nil {
		t.Fatal("Remover should fail when the rename fails")
	}
	if n := len(a.Listar()); n != 1 {
		t.Errorf("memory has %d records after a failed write, want 1", n)
	}
}
