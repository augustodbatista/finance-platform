// Command app is the Finance Platform MVP (ADR-0001): one binary that serves
// the page and stores entries in a JSON file.
//
// Configuration via environment variables (see the README):
//
//	FINANCE_DIA_FECHAMENTO  required, card closing day, 1..31
//	FINANCE_ENDERECO        listen address, default 127.0.0.1:8080 (this machine only)
//	FINANCE_DADOS           data file, default dados.json
//	FINANCE_SENHA           password, required off loopback, at least 8 characters
//
// Operator-facing messages are in Portuguese, like the rest of the UI.
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

// carregar reads and validates the configuration. It takes getenv instead of
// calling os.Getenv so it can be tested without touching the process
// environment.
//
// It fails early, with a message that says what to do: bad config found at
// startup costs a restart; found mid-use, it costs a wrong number on screen.
func carregar(getenv func(string) string) (config, error) {
	c := config{
		endereco: padrao(getenv("FINANCE_ENDERECO"), "127.0.0.1:8080"),
		dados:    padrao(getenv("FINANCE_DADOS"), "dados.json"),
		senha:    getenv("FINANCE_SENHA"),
	}

	dia, err := strconv.Atoi(getenv("FINANCE_DIA_FECHAMENTO"))
	if err != nil || dia < 1 || dia > 31 {
		return config{}, errors.New("FINANCE_DIA_FECHAMENTO deve ser o dia de fechamento do cartão, de 1 a 31")
	}
	c.diaFechamento = dia

	host, _, err := net.SplitHostPort(c.endereco)
	if err != nil {
		return config{}, fmt.Errorf("FINANCE_ENDERECO inválido (%q): use host:porta, ex. 0.0.0.0:8080", c.endereco)
	}

	if !loopback(host) {
		if c.senha == "" {
			return config{}, fmt.Errorf("FINANCE_SENHA é obrigatória ao escutar em %s: sem ela, qualquer um na mesma rede vê seus dados", c.endereco)
		}
		if len(c.senha) < 8 {
			return config{}, errors.New("FINANCE_SENHA deve ter pelo menos 8 caracteres")
		}
	}
	return c, nil
}

// loopback reports whether the host only accepts connections from this
// machine. An empty host (":8080") listens on every interface, so it is NOT
// loopback, even though it looks local.
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
		// Explicit timeouts: without them a slow client holds connections
		// forever (and gosec fails the build).
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("dados em %s, fechamento dia %d", c.dados, c.diaFechamento)
	anunciar(c.endereco)
	log.Fatal(srv.ListenAndServe())
}

// anunciar prints the addresses to open on the phone. When the server listens
// on every interface, the useful address is the machine's local network IP,
// which the user does not know by heart.
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
