package armazem_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/armazem"
	"github.com/augustodbatista/finance-platform/api/internal/dominio"
	"github.com/augustodbatista/finance-platform/api/internal/fatura"
	"github.com/augustodbatista/finance-platform/api/internal/parser"
)

var agora = time.Date(2026, time.September, 25, 10, 0, 0, 0, time.UTC)

func caminho(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "dados.json")
}

func lanc(t *testing.T, texto string) dominio.Lancamento {
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

// testdata/formato-v1.json was written by the code of September 2026 and is
// the format of every real dados.json out there. Lancamento is serialized
// without JSON tags, so renaming or moving a field would make old files load
// with that field silently zeroed. This test fails first.
func TestFormatoV1ContinuaLegivel(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "formato-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	p := caminho(t)
	// gosec G703 flags this as path traversal because the bytes come from a
	// file. False positive: p is inside t.TempDir() and no user input is involved.
	if err := os.WriteFile(p, b, 0o600); err != nil { //nolint:gosec
		t.Fatal(err)
	}

	lista := abrir(t, p).Listar() // newest first
	if len(lista) != 3 {
		t.Fatalf("%d records, want 3", len(lista))
	}

	brt := time.FixedZone("BRT", -3*60*60)
	want := []struct {
		id        int64
		texto     string
		centavos  int64
		categoria string
		tipo      string
		data      time.Time
		forma     string
		parcelas  int
	}{
		{3, "almoço 42,50 ontem", 4250, "alimentacao", "despesa", time.Date(2026, time.September, 24, 0, 0, 0, 0, brt), "", 1},
		{2, "3x 1.200 curso credito 12/09", 120000, "educacao", "despesa", time.Date(2026, time.September, 12, 0, 0, 0, 0, brt), "credito", 3},
		{1, "salário 3500 pix", 350000, "salario", "receita", time.Date(2026, time.September, 25, 0, 0, 0, 0, brt), "pix", 1},
	}
	for i, w := range want {
		got := lista[i]
		l := got.Lancamento
		if got.ID != w.id || got.Texto != w.texto || l.Centavos != w.centavos ||
			string(l.Categoria) != w.categoria || string(l.Tipo) != w.tipo ||
			!l.Data.Equal(w.data) || string(l.Forma) != w.forma || l.Parcelas != w.parcelas {
			t.Errorf("record %d = %+v, want %+v", i, got, w)
		}
	}

	// The next ID survives too: a new entry must not reuse an old one.
	r, err := abrir(t, p).Adicionar("10 mercado", lanc(t, "10 mercado"))
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != 4 {
		t.Errorf("new ID = %d, want 4 (proximo_id from the file)", r.ID)
	}
}

// Entries that are not bills must be written exactly as before bills existed:
// no Vencimento or Pagamento keys. That keeps files readable by older builds
// and keeps the format test meaningful.
func TestLancamentoComumNaoGravaCamposDeConta(t *testing.T) {
	p := caminho(t)
	if _, err := abrir(t, p).Adicionar("10 mercado", lanc(t, "10 mercado")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Clean(p))
	if err != nil {
		t.Fatal(err)
	}
	for _, chave := range []string{`"Vencimento"`, `"Pagamento"`} {
		if strings.Contains(string(b), chave) {
			t.Errorf("file contains %s for a regular entry:\n%s", chave, b)
		}
	}
}

func contaPaga(t *testing.T) dominio.Lancamento {
	t.Helper()
	brt := time.FixedZone("BRT", -3*60*60)
	conta, err := lanc(t, "luz 180 pix").ComoConta(time.Date(2026, time.October, 10, 0, 0, 0, 0, brt))
	if err != nil {
		t.Fatal(err)
	}
	paga, err := conta.Pagar(time.Date(2026, time.October, 12, 0, 0, 0, 0, brt), 18540,
		time.Date(2026, time.October, 12, 9, 0, 0, 0, brt))
	if err != nil {
		t.Fatal(err)
	}
	return paga
}

func TestContaPagaPersisteEntreAberturas(t *testing.T) {
	p := caminho(t)
	l := contaPaga(t)
	if _, err := abrir(t, p).Adicionar("luz 180 pix", l); err != nil {
		t.Fatal(err)
	}

	got := abrir(t, p).Listar()[0].Lancamento
	if !got.Vencimento.Equal(l.Vencimento) || !got.Pagamento.Data.Equal(l.Pagamento.Data) ||
		got.Pagamento.Centavos != 18540 || !got.EConta() || !got.Pago() {
		t.Errorf("reopened bill = %+v, want %+v", got, l)
	}
}

func TestAlterar(t *testing.T) {
	p := caminho(t)
	a := abrir(t, p)
	r, _ := a.Adicionar("luz 180 pix", lanc(t, "luz 180 pix"))
	venc := time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC)

	alterado, err := a.Alterar(r.ID, func(l dominio.Lancamento) (dominio.Lancamento, error) {
		return l.ComoConta(venc)
	})
	if err != nil {
		t.Fatalf("Alterar: %v", err)
	}
	if !alterado.Lancamento.Vencimento.Equal(venc) || alterado.ID != r.ID || alterado.Texto != r.Texto {
		t.Errorf("returned record = %+v", alterado)
	}
	if got := abrir(t, p).Listar()[0].Lancamento; !got.Vencimento.Equal(venc) {
		t.Errorf("after reopening, Vencimento = %v, want %v", got.Vencimento, venc)
	}
}

func TestAlterar_Recusas(t *testing.T) {
	p := caminho(t)
	a := abrir(t, p)
	r, _ := a.Adicionar("luz 180 pix", lanc(t, "luz 180 pix"))
	regra := errors.New("rule refused")

	if _, err := a.Alterar(999, func(l dominio.Lancamento) (dominio.Lancamento, error) { return l, nil }); !errors.Is(err, armazem.ErrNaoEncontrado) {
		t.Errorf("unknown ID: error = %v, want ErrNaoEncontrado", err)
	}

	// The domain refuses: nothing changes, in memory or on disk, and the
	// domain's error reaches the caller untouched.
	_, err := a.Alterar(r.ID, func(l dominio.Lancamento) (dominio.Lancamento, error) {
		l.Centavos = 1
		return l, regra
	})
	if !errors.Is(err, regra) {
		t.Errorf("error = %v, want the domain's error", err)
	}
	if a.Listar()[0].Lancamento.Centavos != 18000 || abrir(t, p).Listar()[0].Lancamento.Centavos != 18000 {
		t.Error("a refused change altered the entry")
	}
}

func TestAlterar_FalhaDeGravacaoNaoAlteraMemoria(t *testing.T) {
	p := caminho(t)
	a := abrir(t, p)
	r, _ := a.Adicionar("luz 180 pix", lanc(t, "luz 180 pix"))
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := a.Alterar(r.ID, func(l dominio.Lancamento) (dominio.Lancamento, error) {
		l.Centavos = 1
		return l, nil
	})
	if err == nil {
		t.Fatal("Alterar should fail when the rename fails")
	}
	if a.Listar()[0].Lancamento.Centavos != 18000 {
		t.Error("memory changed after a failed write")
	}
}

func pagamentoFatura(t *testing.T, mes time.Month, centavos int64) fatura.Pagamento {
	t.Helper()
	hoje := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)
	p, err := fatura.Pagar(fatura.Competencia{Ano: 2026, Mes: mes},
		time.Date(2026, mes+1, 10, 0, 0, 0, 0, time.UTC), centavos, hoje)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPagarFatura_PersisteECorrige(t *testing.T) {
	p := caminho(t)
	a := abrir(t, p)
	if n := len(a.FaturasPagas()); n != 0 {
		t.Fatalf("new store has %d paid statements, want 0", n)
	}

	if err := a.PagarFatura(pagamentoFatura(t, time.September, 123456)); err != nil {
		t.Fatal(err)
	}
	if err := a.PagarFatura(pagamentoFatura(t, time.August, 5000)); err != nil {
		t.Fatal(err)
	}
	// Paying the same statement again corrects it instead of adding a second payment.
	if err := a.PagarFatura(pagamentoFatura(t, time.September, 130000)); err != nil {
		t.Fatal(err)
	}

	pagas := abrir(t, p).FaturasPagas()
	if len(pagas) != 2 {
		t.Fatalf("%d paid statements after reopening, want 2: %+v", len(pagas), pagas)
	}
	porMes := map[time.Month]int64{}
	for _, pg := range pagas {
		porMes[pg.Competencia.Mes] = pg.Centavos
	}
	if porMes[time.September] != 130000 || porMes[time.August] != 5000 {
		t.Errorf("paid statements = %v; want September corrected to 130000 and August 5000", porMes)
	}
}

func TestDesfazerPagamentoFatura(t *testing.T) {
	p := caminho(t)
	a := abrir(t, p)
	setembro := fatura.Competencia{Ano: 2026, Mes: time.September}
	if err := a.PagarFatura(pagamentoFatura(t, time.September, 123456)); err != nil {
		t.Fatal(err)
	}

	if err := a.DesfazerPagamentoFatura(setembro); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if n := len(abrir(t, p).FaturasPagas()); n != 0 {
		t.Errorf("%d paid statements after undo and reopen, want 0", n)
	}
	if err := a.DesfazerPagamentoFatura(setembro); !errors.Is(err, armazem.ErrNaoEncontrado) {
		t.Errorf("undoing an unpaid statement: error = %v, want ErrNaoEncontrado", err)
	}
}

func TestFaturasPagasDevolveCopia(t *testing.T) {
	a := abrir(t, caminho(t))
	if err := a.PagarFatura(pagamentoFatura(t, time.September, 1000)); err != nil {
		t.Fatal(err)
	}
	a.FaturasPagas()[0].Centavos = 1
	if a.FaturasPagas()[0].Centavos != 1000 {
		t.Error("changing the returned slice altered internal state")
	}
}

// Until a statement is paid, the file is written exactly as before.
func TestSemFaturaPagaArquivoNaoMuda(t *testing.T) {
	p := caminho(t)
	if _, err := abrir(t, p).Adicionar("10 mercado", lanc(t, "10 mercado")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Clean(p))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "faturas_pagas") {
		t.Errorf("file has faturas_pagas with no paid statement:\n%s", b)
	}
}

func TestPagarFatura_FalhaDeGravacaoNaoAlteraMemoria(t *testing.T) {
	p := caminho(t)
	a := abrir(t, p)
	if _, err := a.Adicionar("10 mercado", lanc(t, "10 mercado")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := a.PagarFatura(pagamentoFatura(t, time.September, 1000)); err == nil {
		t.Fatal("PagarFatura should fail when the rename fails")
	}
	if n := len(a.FaturasPagas()); n != 0 {
		t.Errorf("memory has %d paid statements after a failed write, want 0", n)
	}
	if err := a.DesfazerPagamentoFatura(fatura.Competencia{Ano: 2026, Mes: time.September}); !errors.Is(err, armazem.ErrNaoEncontrado) {
		t.Errorf("undo after failed pay: error = %v, want ErrNaoEncontrado", err)
	}
}

// Adding, changing or removing an entry rewrites the whole file: it must carry
// the paid statements along instead of dropping them.
func TestOperacoesEmLancamentosPreservamFaturasPagas(t *testing.T) {
	p := caminho(t)
	a := abrir(t, p)
	if err := a.PagarFatura(pagamentoFatura(t, time.September, 123456)); err != nil {
		t.Fatal(err)
	}

	r, err := a.Adicionar("10 mercado", lanc(t, "10 mercado"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Alterar(r.ID, func(l dominio.Lancamento) (dominio.Lancamento, error) { return l, nil }); err != nil {
		t.Fatal(err)
	}
	if err := a.Remover(r.ID); err != nil {
		t.Fatal(err)
	}

	if n := len(abrir(t, p).FaturasPagas()); n != 1 {
		t.Errorf("%d paid statements after add/change/remove and reopen, want 1", n)
	}
}
