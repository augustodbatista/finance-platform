package parser

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrDataInvalida reports a date that matches the format but does not exist on
// the calendar.
var ErrDataInvalida = errors.New("parser: invalid date")

// reData matches dd/mm, dd/mm/yy and dd/mm/yyyy.
//
// The 4-digit year comes first in the alternation on purpose: with \d{2} first,
// "2026" would match as "20" and leave "26" loose in the string -- which the
// amount extractor would then swallow as if it were money.
var reData = regexp.MustCompile(`\b(\d{1,2})/(\d{1,2})(?:/(\d{4}|\d{2}))?\b`)

// reRelativa matches the relative date words ("today", "yesterday"), already
// normalized.
var reRelativa = regexp.MustCompile(`\b(hoje|ontem)\b`)

// extrairData returns the purchase date and the input without the date token.
//
// It expects normalized input (see normalizar), because it matches words by
// exact text: "ONTEM" would not match the lowercase pattern.
//
// Returning the cleaned input is not a convenience, it is correctness. The
// "last number wins" rule would turn "mercado 120 15/03" into R$ 0,03 if the
// date token were still in the string when the amount is extracted.
//
// The date is truncated to midnight: to know which statement a purchase landed
// on, the time of day adds nothing and would only get in the way of comparing.
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

// dataExplicita builds the date from the groups matched by reData.
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

	// time.Date normalizes instead of refusing: 30/02 becomes 02/03. Comparing
	// back is the standard library's way to detect a date that does not exist.
	if data.Year() != ano || data.Month() != time.Month(mes) || data.Day() != dia {
		return time.Time{}, ErrDataInvalida
	}

	// With no year given, dd/mm resolves to the most recent past occurrence:
	// on January 15, "20/12" is December of the previous year. Logging late is
	// routine; logging with a future date is almost always a mistake.
	if m[3] == "" && data.After(hoje) {
		data = data.AddDate(-1, 0, 0)
	}

	return data, nil
}
