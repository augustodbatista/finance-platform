// Package armazem guarda os lancamentos num arquivo JSON local.
//
// ponytail: um arquivo JSON inteiro reescrito a cada mudanca. Serve bem para os
// milhares de lancamentos de uma pessoa; acima de ~5MB ou com lentidao, troca por
// SQLite (gatilho 3 do ADR-0001).
package armazem

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/augustodbatista/finance-platform/api/internal/parser"
)

// ErrNaoEncontrado indica ID que nao existe (ou ja foi removido).
var ErrNaoEncontrado = errors.New("armazem: lancamento nao encontrado")

// Registro e um lancamento salvo, com o texto que o usuario digitou.
//
// O texto original e guardado porque e a melhor descricao que existe: o usuario
// reconhece "almoco com o time 42,50" na lista, nao "Alimentacao R$ 42,50".
type Registro struct {
	ID         int64             `json:"id"`
	Texto      string            `json:"texto"`
	Lancamento parser.Lancamento `json:"lancamento"`
}

type conteudo struct {
	// ProximoID persiste separado do maior ID existente: sem ele, remover o
	// ultimo registro e reabrir faria o ID ser reaproveitado, e um DELETE
	// atrasado apagaria o registro errado.
	ProximoID int64      `json:"proximo_id"`
	Registros []Registro `json:"registros"`
}

// Armazem e seguro para uso concorrente.
type Armazem struct {
	mu      sync.Mutex
	caminho string
	dados   conteudo
}

// Abrir carrega o arquivo, ou comeca vazio se ele nao existir.
//
// Arquivo que existe mas nao e JSON valido e erro, nunca "comecar vazio": o
// proximo Adicionar sobrescreveria o arquivo e apagaria tudo o que estava la.
// O arquivo fica intacto para recuperacao manual.
func Abrir(caminho string) (*Armazem, error) {
	a := &Armazem{caminho: filepath.Clean(caminho), dados: conteudo{ProximoID: 1}}

	b, err := os.ReadFile(a.caminho)
	if errors.Is(err, os.ErrNotExist) {
		return a, nil
	}
	if err != nil {
		return nil, fmt.Errorf("armazem: lendo %s: %w", a.caminho, err)
	}
	if err := json.Unmarshal(b, &a.dados); err != nil {
		return nil, fmt.Errorf("armazem: %s nao e JSON valido (arquivo mantido intacto): %w", a.caminho, err)
	}
	return a, nil
}

// Adicionar grava um lancamento novo e devolve o registro com ID.
func (a *Armazem) Adicionar(texto string, l parser.Lancamento) (Registro, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	r := Registro{ID: a.dados.ProximoID, Texto: texto, Lancamento: l}
	novo := conteudo{
		ProximoID: a.dados.ProximoID + 1,
		Registros: append(slices.Clone(a.dados.Registros), r),
	}
	if err := a.gravar(novo); err != nil {
		return Registro{}, err
	}
	a.dados = novo
	return r, nil
}

// Listar devolve uma copia dos registros, do mais recente para o mais antigo.
func (a *Armazem) Listar() []Registro {
	a.mu.Lock()
	defer a.mu.Unlock()

	rs := slices.Clone(a.dados.Registros)
	slices.Reverse(rs)
	return rs
}

// Remover apaga o registro com o ID informado.
func (a *Armazem) Remover(id int64) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	i := slices.IndexFunc(a.dados.Registros, func(r Registro) bool { return r.ID == id })
	if i < 0 {
		return ErrNaoEncontrado
	}
	novo := conteudo{
		ProximoID: a.dados.ProximoID,
		Registros: slices.Delete(slices.Clone(a.dados.Registros), i, i+1),
	}
	if err := a.gravar(novo); err != nil {
		return err
	}
	a.dados = novo
	return nil
}

// gravar escreve de forma atomica: arquivo temporario no mesmo diretorio e
// rename por cima. Uma queda no meio da escrita deixa o arquivo antigo inteiro,
// nunca um arquivo pela metade. O estado em memoria so muda depois que o disco
// confirmou -- se gravar falha, memoria e disco continuam iguais.
//
// Permissao 0600: sao dados financeiros, ninguem alem do dono le.
func (a *Armazem) gravar(c conteudo) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("armazem: serializando: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(a.caminho), ".dados-*.tmp")
	if err != nil {
		return fmt.Errorf("armazem: criando temporario: %w", err)
	}
	// Depois de um rename bem-sucedido o temporario ja nao existe e o Remove
	// falha de proposito; o erro nao carrega informacao, por isso e descartado.
	defer func() { _ = os.Remove(tmp.Name()) }()

	// errors.Join em vez de ignorar o Close: se as duas coisas falharem, as
	// duas aparecem no log.
	if _, err := tmp.Write(b); err != nil {
		return errors.Join(fmt.Errorf("armazem: escrevendo temporario: %w", err), tmp.Close())
	}
	if err := tmp.Sync(); err != nil {
		return errors.Join(fmt.Errorf("armazem: sincronizando temporario: %w", err), tmp.Close())
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("armazem: fechando temporario: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return fmt.Errorf("armazem: ajustando permissao: %w", err)
	}
	if err := os.Rename(tmp.Name(), a.caminho); err != nil {
		return fmt.Errorf("armazem: substituindo %s: %w", a.caminho, err)
	}
	return nil
}
