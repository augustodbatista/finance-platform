package parser

import (
	"strings"

	"github.com/augustodbatista/finance-platform/api/internal/dominio"
)

// termos maps a typed word to a category. Keys are already normalized
// (lowercase, no accents), because input goes through normalizar() first.
//
// ponytail: exact word-by-word lookup. No stemming, plurals or typo
// correction. If categories are often wrong in practice, the next step is edit
// distance over these keys -- not an LLM, which would add latency and cost to
// every entry (RFC-0001).
var termos = map[string]dominio.Categoria{
	"alimentacao": dominio.Alimentacao,
	"almoco":      dominio.Alimentacao,
	"jantar":      dominio.Alimentacao,
	"cafe":        dominio.Alimentacao,
	"lanche":      dominio.Alimentacao,
	"restaurante": dominio.Alimentacao,
	"ifood":       dominio.Alimentacao,
	"padaria":     dominio.Alimentacao,
	"pizza":       dominio.Alimentacao,

	"mercado":      dominio.Mercado,
	"supermercado": dominio.Mercado,
	"feira":        dominio.Mercado,
	"hortifruti":   dominio.Mercado,
	"acougue":      dominio.Mercado,

	"transporte":     dominio.Transporte,
	"uber":           dominio.Transporte,
	"taxi":           dominio.Transporte,
	"onibus":         dominio.Transporte,
	"metro":          dominio.Transporte,
	"gasolina":       dominio.Transporte,
	"combustivel":    dominio.Transporte,
	"estacionamento": dominio.Transporte,

	"casa":       dominio.Casa,
	"aluguel":    dominio.Casa,
	"condominio": dominio.Casa,
	"luz":        dominio.Casa,
	"agua":       dominio.Casa,
	"gas":        dominio.Casa,
	"internet":   dominio.Casa,

	"saude":    dominio.Saude,
	"farmacia": dominio.Saude,
	"remedio":  dominio.Saude,
	"medico":   dominio.Saude,
	"dentista": dominio.Saude,

	"lazer":  dominio.Lazer,
	"cinema": dominio.Lazer,
	"bar":    dominio.Lazer,
	"show":   dominio.Lazer,
	"viagem": dominio.Lazer,

	"educacao":    dominio.Educacao,
	"curso":       dominio.Educacao,
	"faculdade":   dominio.Educacao,
	"livro":       dominio.Educacao,
	"mensalidade": dominio.Educacao,

	"assinaturas": dominio.Assinaturas,
	"assinatura":  dominio.Assinaturas,
	"netflix":     dominio.Assinaturas,
	"spotify":     dominio.Assinaturas,
	"disney":      dominio.Assinaturas,
	"youtube":     dominio.Assinaturas,
	"hbo":         dominio.Assinaturas,

	"salario":    dominio.Salario,
	"pagamento":  dominio.Salario,
	"freelancer": dominio.Freelancer,
	"freela":     dominio.Freelancer,

	"investimentos": dominio.Investimentos,
	"investimento":  dominio.Investimentos,
	"dividendo":     dominio.Investimentos,
	"rendimento":    dominio.Investimentos,
	"juros":         dominio.Investimentos,
}

// semAcento replaces Portuguese accented letters with their base letter, so
// "almoço" and "almoco" land on the same category.
//
// ponytail: a standard library Replacer instead of
// golang.org/x/text/unicode/norm. About 20 pairs cover all of Portuguese; the
// external dependency would only pay off if we had to normalize languages we
// do not control.
var semAcento = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

// normalizar makes text comparable with the termos keys and with the package's
// word patterns. Parse calls it once, up front, and the rest of the pipeline
// works on the result -- normalizing in two different places is what used to
// make the intermediate string come out sometimes original, sometimes
// normalized, depending on the path taken.
func normalizar(s string) string {
	return semAcento.Replace(strings.ToLower(s))
}

// classificar returns the category of the first recognized word in the input.
// No recognized word returns Outros, never an error.
//
// Expects normalized input (see normalizar).
func classificar(entrada string) dominio.Categoria {
	for _, palavra := range strings.Fields(entrada) {
		if c, ok := termos[palavra]; ok {
			return c
		}
	}
	return dominio.Outros
}
