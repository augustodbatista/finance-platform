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
		t.Errorf("Listar() tem %d registros, quero 0", n)
	}
}

// O teste que importa: o que foi gravado sobrevive a fechar e reabrir, campo por
// campo. Se isso falhar, o app perde dinheiro registrado.
func TestAdicionar_PersisteEntreAberturas(t *testing.T) {
	p := caminho(t)
	l := lanc(t, "3x 300 mercado credito 15/09")

	r, err := abrir(t, p).Adicionar("3x 300 mercado credito 15/09", l)
	if err != nil {
		t.Fatalf("Adicionar: %v", err)
	}

	lista := abrir(t, p).Listar()
	if len(lista) != 1 {
		t.Fatalf("depois de reabrir, %d registros, quero 1", len(lista))
	}
	got := lista[0]
	if got.ID != r.ID || got.Texto != "3x 300 mercado credito 15/09" {
		t.Errorf("registro = %+v, quero ID %d e o texto original", got, r.ID)
	}
	gl := got.Lancamento
	if gl.Centavos != l.Centavos || gl.Categoria != l.Categoria || gl.Tipo != l.Tipo ||
		gl.Forma != l.Forma || gl.Parcelas != l.Parcelas || !gl.Data.Equal(l.Data) {
		t.Errorf("lancamento reaberto = %+v, quero %+v", gl, l)
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
		t.Errorf("ordem = %q, %q, %q; quero o mais recente primeiro",
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
		t.Error("mexer no slice devolvido alterou o estado interno")
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
		t.Errorf("depois de remover e reabrir: %+v, quero so o ID %d", lista, r2.ID)
	}

	if err := a.Remover(r1.ID); !errors.Is(err, armazem.ErrNaoEncontrado) {
		t.Errorf("remover de novo: erro = %v, quero ErrNaoEncontrado", err)
	}
}

// ID reaproveitado faria um DELETE atrasado apagar o registro errado.
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
		t.Errorf("ID %d reaproveitado (existentes/removidos: %d, %d)", r3.ID, r1.ID, r2.ID)
	}
}

// Arquivo ilegivel nao pode virar "comecar vazio": o proximo Adicionar
// sobrescreveria o arquivo e apagaria tudo o que estava la.
func TestAbrir_ArquivoCorrompidoFalhaEmVezDeZerar(t *testing.T) {
	p := caminho(t)
	if err := os.WriteFile(p, []byte("{nao e json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := armazem.Abrir(p); err == nil {
		t.Fatal("Abrir de arquivo corrompido devia falhar")
	}
	if b, _ := os.ReadFile(filepath.Clean(p)); string(b) != "{nao e json" {
		t.Error("Abrir alterou o arquivo corrompido; devia deixa-lo intacto para recuperacao")
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
		t.Errorf("diretorio tem %d arquivos, quero so o de dados (temporario esquecido?)", len(entradas))
	}
}

// Handlers HTTP rodam em paralelo. Sem trava, gravacoes simultaneas perdem
// lancamentos -- e o -race do CI acusa a corrida.
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
		t.Errorf("%d registros persistidos, quero 50", n)
	}
}

func TestAbrir_ErroDeLeituraNaoViraVazio(t *testing.T) {
	// Um diretorio no lugar do arquivo: existe, mas nao da para ler.
	if _, err := armazem.Abrir(t.TempDir()); err == nil {
		t.Fatal("Abrir de caminho ilegivel devia falhar, nao comecar vazio")
	}
}

// Se o disco recusa a gravacao, memoria e disco tem que continuar iguais.
// Senao a tela mostra um lancamento que sumira no proximo reinicio.
func TestAdicionar_FalhaDeGravacaoNaoAlteraMemoria(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nao-existe", "dados.json")
	a := abrir(t, p)

	if _, err := a.Adicionar("10 mercado", lanc(t, "10 mercado")); err == nil {
		t.Fatal("Adicionar em diretorio inexistente devia falhar")
	}
	if n := len(a.Listar()); n != 0 {
		t.Errorf("memoria tem %d registros apos falha de gravacao, quero 0", n)
	}
}

func TestRemover_FalhaDeGravacaoNaoAlteraMemoria(t *testing.T) {
	p := caminho(t)
	a := abrir(t, p)
	r, _ := a.Adicionar("10 mercado", lanc(t, "10 mercado"))

	// Troca o arquivo por um diretorio: o rename final passa a falhar.
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := a.Remover(r.ID); err == nil {
		t.Fatal("Remover devia falhar quando o rename falha")
	}
	if n := len(a.Listar()); n != 1 {
		t.Errorf("memoria tem %d registros apos falha de gravacao, quero 1", n)
	}
}
