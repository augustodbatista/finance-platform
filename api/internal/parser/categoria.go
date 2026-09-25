package parser

import "strings"

// Categoria is a closed set, not free text. Unrecognized input falls into
// Outros (other) and never becomes an error: the product thesis is that
// friction kills habit, and asking "which category?" costs more than
// classifying wrong.
type Categoria string

const (
	// Expenses.
	Alimentacao Categoria = "alimentacao"
	Mercado     Categoria = "mercado"
	Transporte  Categoria = "transporte"
	Casa        Categoria = "casa"
	Saude       Categoria = "saude"
	Lazer       Categoria = "lazer"
	Educacao    Categoria = "educacao"
	Assinaturas Categoria = "assinaturas"

	// Income.
	Salario       Categoria = "salario"
	Freelancer    Categoria = "freelancer"
	Investimentos Categoria = "investimentos"

	// Valid for both income and expenses.
	Outros Categoria = "outros"
)

// termos maps a typed word to a category. Keys are already normalized
// (lowercase, no accents), because input goes through normalizar() first.
//
// ponytail: exact word-by-word lookup. No stemming, plurals or typo
// correction. If categories are often wrong in practice, the next step is edit
// distance over these keys -- not an LLM, which would add latency and cost to
// every entry (RFC-0001).
var termos = map[string]Categoria{
	"alimentacao": Alimentacao,
	"almoco":      Alimentacao,
	"jantar":      Alimentacao,
	"cafe":        Alimentacao,
	"lanche":      Alimentacao,
	"restaurante": Alimentacao,
	"ifood":       Alimentacao,
	"padaria":     Alimentacao,
	"pizza":       Alimentacao,

	"mercado":      Mercado,
	"supermercado": Mercado,
	"feira":        Mercado,
	"hortifruti":   Mercado,
	"acougue":      Mercado,

	"transporte":     Transporte,
	"uber":           Transporte,
	"taxi":           Transporte,
	"onibus":         Transporte,
	"metro":          Transporte,
	"gasolina":       Transporte,
	"combustivel":    Transporte,
	"estacionamento": Transporte,

	"casa":       Casa,
	"aluguel":    Casa,
	"condominio": Casa,
	"luz":        Casa,
	"agua":       Casa,
	"gas":        Casa,
	"internet":   Casa,

	"saude":    Saude,
	"farmacia": Saude,
	"remedio":  Saude,
	"medico":   Saude,
	"dentista": Saude,

	"lazer":  Lazer,
	"cinema": Lazer,
	"bar":    Lazer,
	"show":   Lazer,
	"viagem": Lazer,

	"educacao":    Educacao,
	"curso":       Educacao,
	"faculdade":   Educacao,
	"livro":       Educacao,
	"mensalidade": Educacao,

	"assinaturas": Assinaturas,
	"assinatura":  Assinaturas,
	"netflix":     Assinaturas,
	"spotify":     Assinaturas,
	"disney":      Assinaturas,
	"youtube":     Assinaturas,
	"hbo":         Assinaturas,

	"salario":    Salario,
	"pagamento":  Salario,
	"freelancer": Freelancer,
	"freela":     Freelancer,

	"investimentos": Investimentos,
	"investimento":  Investimentos,
	"dividendo":     Investimentos,
	"rendimento":    Investimentos,
	"juros":         Investimentos,
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
func classificar(entrada string) Categoria {
	for _, palavra := range strings.Fields(entrada) {
		if c, ok := termos[palavra]; ok {
			return c
		}
	}
	return Outros
}

// receitas are the categories that represent money coming in.
var receitas = map[Categoria]bool{
	Salario:       true,
	Freelancer:    true,
	Investimentos: true,
}

// tipoDe derives income or expense from the category, so the user never has to
// declare it: typing "salario 3500" already says everything needed.
//
// Outros falls into Despesa (expense). It is ambiguous by definition (it exists
// in both lists), and the overwhelming majority of entries are money going out
// -- the default that is wrong the least.
func tipoDe(c Categoria) Tipo {
	if receitas[c] {
		return Receita
	}
	return Despesa
}
