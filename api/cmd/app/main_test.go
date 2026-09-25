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
		t.Fatalf("erro inesperado: %v", err)
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
		{"sem dia de fechamento", map[string]string{}, "FINANCE_DIA_FECHAMENTO"},
		{"dia nao numerico", map[string]string{"FINANCE_DIA_FECHAMENTO": "vinte"}, "FINANCE_DIA_FECHAMENTO"},
		{"dia fora da faixa", map[string]string{"FINANCE_DIA_FECHAMENTO": "32"}, "FINANCE_DIA_FECHAMENTO"},
		{"endereco invalido", map[string]string{"FINANCE_DIA_FECHAMENTO": "28", "FINANCE_ENDERECO": "sem-porta"}, "FINANCE_ENDERECO"},
		// O motivo deste teste existir: expor na rede sem senha entregaria os
		// dados financeiros a qualquer um na mesma wifi.
		{"rede sem senha", map[string]string{"FINANCE_DIA_FECHAMENTO": "28", "FINANCE_ENDERECO": "0.0.0.0:8080"}, "FINANCE_SENHA"},
		// ":8080" sem host escuta em TODAS as interfaces -- parece local, nao e.
		{"host vazio sem senha", map[string]string{"FINANCE_DIA_FECHAMENTO": "28", "FINANCE_ENDERECO": ":8080"}, "FINANCE_SENHA"},
		{"senha curta", map[string]string{"FINANCE_DIA_FECHAMENTO": "28", "FINANCE_ENDERECO": "0.0.0.0:8080", "FINANCE_SENHA": "1234"}, "8 caracteres"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, err := carregar(env(c.env))
			if err == nil || !strings.Contains(err.Error(), c.trecho) {
				t.Errorf("erro = %v, quero mencionando %q", err, c.trecho)
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
			t.Errorf("%v: erro inesperado %v", e, err)
		}
	}
}
