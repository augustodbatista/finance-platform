// Comando app e o MVP do Finance Platform (ADR-0001): um binario que serve a
// pagina e guarda os lancamentos num arquivo JSON.
//
// Configuracao por variavel de ambiente (ver tabela no CLAUDE.md):
//
//	FINANCE_DIA_FECHAMENTO  obrigatoria, 1..31
//	FINANCE_ENDERECO        padrao 127.0.0.1:8080 (so esta maquina)
//	FINANCE_DADOS           padrao dados.json
//	FINANCE_SENHA           obrigatoria fora do loopback, minimo 8 caracteres
package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/armazem"
	"github.com/augustodbatista/finance-platform/api/internal/web"
)

type config struct {
	endereco      string
	dados         string
	senha         string
	diaFechamento int
}

// carregar le e valida a configuracao. Recebe getenv em vez de chamar
// os.Getenv para ser testavel sem mexer no ambiente do processo.
//
// Falha cedo e com mensagem que diz o que fazer: config errada descoberta na
// subida custa um reinicio; descoberta no meio do uso custa um numero errado
// na tela.
func carregar(getenv func(string) string) (config, error) {
	c := config{
		endereco: padrao(getenv("FINANCE_ENDERECO"), "127.0.0.1:8080"),
		dados:    padrao(getenv("FINANCE_DADOS"), "dados.json"),
		senha:    getenv("FINANCE_SENHA"),
	}

	dia, err := strconv.Atoi(getenv("FINANCE_DIA_FECHAMENTO"))
	if err != nil || dia < 1 || dia > 31 {
		return config{}, errors.New("FINANCE_DIA_FECHAMENTO deve ser o dia de fechamento do cartao, de 1 a 31")
	}
	c.diaFechamento = dia

	host, _, err := net.SplitHostPort(c.endereco)
	if err != nil {
		return config{}, fmt.Errorf("FINANCE_ENDERECO invalido (%q): use host:porta, ex. 0.0.0.0:8080", c.endereco)
	}

	if !loopback(host) {
		if c.senha == "" {
			return config{}, fmt.Errorf("FINANCE_SENHA e obrigatoria ao escutar em %s: sem ela, qualquer um na mesma rede ve seus dados", c.endereco)
		}
		if len(c.senha) < 8 {
			return config{}, errors.New("FINANCE_SENHA deve ter pelo menos 8 caracteres")
		}
	}
	return c, nil
}

// loopback diz se o host so aceita conexao desta maquina. Host vazio (":8080")
// escuta em todas as interfaces, entao NAO e loopback, apesar de parecer local.
func loopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func padrao(v, p string) string {
	if v == "" {
		return p
	}
	return v
}

func main() {
	c, err := carregar(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}

	a, err := armazem.Abrir(c.dados)
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		Addr: c.endereco,
		Handler: web.Novo(web.Config{
			Armazem:       a,
			DiaFechamento: c.diaFechamento,
			Senha:         c.senha,
			Agora:         time.Now,
		}),
		// Timeouts explicitos: sem eles, um cliente lento segura conexoes para
		// sempre (e o gosec reprova).
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("dados em %s, fechamento dia %d", c.dados, c.diaFechamento)
	anunciar(c.endereco)
	log.Fatal(srv.ListenAndServe())
}

// anunciar imprime os enderecos para abrir no celular. Quando o servidor escuta
// em todas as interfaces, o endereco util e o IP da maquina na rede local, que
// o usuario nao sabe de cabeca.
func anunciar(endereco string) {
	host, porta, _ := net.SplitHostPort(endereco)
	if host != "" && host != "0.0.0.0" && host != "::" {
		log.Printf("abra http://%s", endereco)
		return
	}
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil && !ipn.IP.IsLoopback() {
			log.Printf("abra no celular: http://%s", net.JoinHostPort(ipn.IP.String(), porta))
		}
	}
}
