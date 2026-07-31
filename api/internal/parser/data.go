package parser

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrDataInvalida indica data reconhecida na forma mas inexistente no calendario.
var ErrDataInvalida = errors.New("parser: data invalida")

// reData casa dd/mm, dd/mm/aa e dd/mm/aaaa.
//
// O ano de 4 digitos vem primeiro na alternancia de proposito: com \d{2} na
// frente, "2026" casaria como "20" e sobraria "26" solto na string -- que o
// extrator de valor engoliria como se fosse dinheiro.
var reData = regexp.MustCompile(`\b(\d{1,2})/(\d{1,2})(?:/(\d{4}|\d{2}))?\b`)

// reRelativa casa as palavras de data relativa, ja normalizadas.
var reRelativa = regexp.MustCompile(`\b(hoje|ontem)\b`)

// extrairData devolve a data da compra e a entrada sem o token de data.
//
// Espera a entrada ja normalizada (ver normalizar), porque casa palavra por
// texto exato: "ONTEM" nao casaria com o padrao em minusculas.
//
// Devolver a entrada limpa nao e conveniencia: e correcao. A regra "vale o
// ultimo numero" faria "mercado 120 15/03" virar R$ 0,03 se o token de data
// continuasse na string quando o valor fosse extraido.
//
// A data volta truncada na meia-noite: para saber em que fatura a compra caiu,
// a hora nao acrescenta nada e so atrapalharia comparacao.
func extrairData(entrada string, agora time.Time) (time.Time, string, error) {
	hoje := time.Date(agora.Year(), agora.Month(), agora.Day(), 0, 0, 0, 0, agora.Location())

	if m := reData.FindStringSubmatch(entrada); m != nil {
		data, err := dataExplicita(m, hoje)
		if err != nil {
			return time.Time{}, "", err
		}
		return data, strings.Replace(entrada, m[0], " ", 1), nil
	}

	if m := reRelativa.FindString(entrada); m != "" {
		data := hoje
		if m == "ontem" {
			data = hoje.AddDate(0, 0, -1)
		}
		return data, strings.Replace(entrada, m, " ", 1), nil
	}

	return hoje, entrada, nil
}

// dataExplicita monta a data a partir dos grupos casados por reData.
func dataExplicita(m []string, hoje time.Time) (time.Time, error) {
	dia, _ := strconv.Atoi(m[1])
	mes, _ := strconv.Atoi(m[2])

	ano := hoje.Year()
	switch {
	case len(m[3]) == 4:
		ano, _ = strconv.Atoi(m[3])
	case len(m[3]) == 2:
		aa, _ := strconv.Atoi(m[3])
		ano = 2000 + aa
	}

	data := time.Date(ano, time.Month(mes), dia, 0, 0, 0, 0, hoje.Location())

	// time.Date normaliza em vez de recusar: 30/02 vira 02/03. Comparar de
	// volta e o jeito da stdlib de detectar data que nao existe.
	if data.Year() != ano || data.Month() != time.Month(mes) || data.Day() != dia {
		return time.Time{}, ErrDataInvalida
	}

	// Sem ano informado, dd/mm resolve para a ocorrencia passada mais recente:
	// em 15/01, "20/12" e dezembro do ano anterior. Lancamento atrasado e
	// rotina; lancamento com data no futuro quase sempre e engano.
	if m[3] == "" && data.After(hoje) {
		data = data.AddDate(-1, 0, 0)
	}

	return data, nil
}
