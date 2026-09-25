package main

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestCarregar_Padroes(t *testing.T) {
	c, err := carregar(env(map[string]string{"FINANCE_DIA_FECHAMENTO": "28"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.endereco != "127.0.0.1:8080" || c.dados != "dados.json" || c.diaFechamento != 28 || c.senha != "" {
		t.Errorf("config = %+v", c)
	}
}

func TestCarregar_Erros(t *testing.T) {
	casos := []struct {
		nome   string
		env    map[string]string
		trecho string
	}{
		{"missing closing day", map[string]string{}, "FINANCE_DIA_FECHAMENTO"},
		{"non-numeric day", map[string]string{"FINANCE_DIA_FECHAMENTO": "vinte"}, "FINANCE_DIA_FECHAMENTO"},
		{"day out of range", map[string]string{"FINANCE_DIA_FECHAMENTO": "32"}, "FINANCE_DIA_FECHAMENTO"},
		{"invalid address", map[string]string{"FINANCE_DIA_FECHAMENTO": "28", "FINANCE_ENDERECO": "sem-porta"}, "FINANCE_ENDERECO"},
		// The reason this test exists: exposing the app on the network without
		// a password would hand the financial data to anyone on the same wifi.
		{"network without password", map[string]string{"FINANCE_DIA_FECHAMENTO": "28", "FINANCE_ENDERECO": "0.0.0.0:8080"}, "FINANCE_SENHA"},
		// ":8080" with no host listens on ALL interfaces -- it looks local, it
		// is not.
		{"empty host without password", map[string]string{"FINANCE_DIA_FECHAMENTO": "28", "FINANCE_ENDERECO": ":8080"}, "FINANCE_SENHA"},
		{"short password", map[string]string{"FINANCE_DIA_FECHAMENTO": "28", "FINANCE_ENDERECO": "0.0.0.0:8080", "FINANCE_SENHA": "1234"}, "8 caracteres"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, err := carregar(env(c.env))
			if err == nil || !strings.Contains(err.Error(), c.trecho) {
				t.Errorf("error = %v, want one mentioning %q", err, c.trecho)
			}
		})
	}
}

func TestCarregar_Aceitos(t *testing.T) {
	for _, e := range []map[string]string{
		{"FINANCE_DIA_FECHAMENTO": "1", "FINANCE_ENDERECO": "localhost:9000"},
		{"FINANCE_DIA_FECHAMENTO": "31", "FINANCE_ENDERECO": "[::1]:9000"},
		{"FINANCE_DIA_FECHAMENTO": "28", "FINANCE_ENDERECO": "0.0.0.0:8080", "FINANCE_SENHA": "senha-boa"},
	} {
		if _, err := carregar(env(e)); err != nil {
			t.Errorf("%v: unexpected error %v", e, err)
		}
	}
}
