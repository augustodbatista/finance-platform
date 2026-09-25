// Package armazem stores entries in a local JSON file.
//
// ponytail: one JSON file rewritten in full on every change. It handles the
// thousands of entries of one person well; above ~5MB or once it feels slow,
// switch to SQLite (trigger 3 of ADR-0001).
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

// ErrNaoEncontrado reports an ID that does not exist (or was already removed).
var ErrNaoEncontrado = errors.New("armazem: entry not found")

// Registro is a saved entry, together with the text the user typed.
//
// The original text is kept because it is the best description there is: the
// user recognizes "almoco com o time 42,50" in the list, not "Alimentacao
// R$ 42,50".
type Registro struct {
	ID         int64             `json:"id"`
	Texto      string            `json:"texto"`
	Lancamento parser.Lancamento `json:"lancamento"`
}

type conteudo struct {
	// ProximoID is persisted separately from the highest existing ID: without
	// it, removing the last record and reopening would reuse the ID, and a
	// delayed DELETE would remove the wrong record.
	ProximoID int64      `json:"proximo_id"`
	Registros []Registro `json:"registros"`
}

// Armazem is safe for concurrent use.
type Armazem struct {
	mu      sync.Mutex
	caminho string
	dados   conteudo
}

// Abrir loads the file, or starts empty if it does not exist.
//
// A file that exists but is not valid JSON is an error, never "start empty":
// the next Adicionar would overwrite the file and erase everything in it. The
// file is left untouched for manual recovery.
func Abrir(caminho string) (*Armazem, error) {
	a := &Armazem{caminho: filepath.Clean(caminho), dados: conteudo{ProximoID: 1}}

	b, err := os.ReadFile(a.caminho)
	if errors.Is(err, os.ErrNotExist) {
		return a, nil
	}
	if err != nil {
		return nil, fmt.Errorf("armazem: reading %s: %w", a.caminho, err)
	}
	if err := json.Unmarshal(b, &a.dados); err != nil {
		return nil, fmt.Errorf("armazem: %s is not valid JSON (file left untouched): %w", a.caminho, err)
	}
	return a, nil
}

// Adicionar saves a new entry and returns the record with its ID.
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

// Listar returns a copy of the records, newest first.
func (a *Armazem) Listar() []Registro {
	a.mu.Lock()
	defer a.mu.Unlock()

	rs := slices.Clone(a.dados.Registros)
	slices.Reverse(rs)
	return rs
}

// Remover deletes the record with the given ID.
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

// gravar writes atomically: a temporary file in the same directory, renamed
// over the target. A crash mid-write leaves the old file whole, never a
// half-written one. In-memory state only changes after the disk has confirmed
// -- if gravar fails, memory and disk stay the same.
//
// Permission 0600: this is financial data, nobody but the owner reads it.
func (a *Armazem) gravar(c conteudo) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("armazem: encoding: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(a.caminho), ".dados-*.tmp")
	if err != nil {
		return fmt.Errorf("armazem: creating temp file: %w", err)
	}
	// After a successful rename the temp file no longer exists and Remove fails
	// on purpose; that error carries no information, so it is discarded.
	defer func() { _ = os.Remove(tmp.Name()) }()

	// errors.Join instead of ignoring Close: if both fail, both show up in the
	// log.
	if _, err := tmp.Write(b); err != nil {
		return errors.Join(fmt.Errorf("armazem: writing temp file: %w", err), tmp.Close())
	}
	if err := tmp.Sync(); err != nil {
		return errors.Join(fmt.Errorf("armazem: syncing temp file: %w", err), tmp.Close())
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("armazem: closing temp file: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return fmt.Errorf("armazem: setting permissions: %w", err)
	}
	if err := os.Rename(tmp.Name(), a.caminho); err != nil {
		return fmt.Errorf("armazem: replacing %s: %w", a.caminho, err)
	}
	return nil
}
