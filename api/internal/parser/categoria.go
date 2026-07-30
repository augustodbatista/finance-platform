package parser

import "strings"

// Categoria e um conjunto fechado, nao texto livre. Entrada nao reconhecida cai
// em Outros e nunca vira erro: a tese do produto e que atrito mata habito, e
// perguntar "qual categoria?" custa mais do que classificar errado.
type Categoria string

const (
	// Despesas.
	Alimentacao Categoria = "alimentacao"
	Mercado     Categoria = "mercado"
	Transporte  Categoria = "transporte"
	Casa        Categoria = "casa"
	Saude       Categoria = "saude"
	Lazer       Categoria = "lazer"
	Educacao    Categoria = "educacao"
	Assinaturas Categoria = "assinaturas"

	// Receitas.
	Salario       Categoria = "salario"
	Freelancer    Categoria = "freelancer"
	Investimentos Categoria = "investimentos"

	// Vale para receita e despesa.
	Outros Categoria = "outros"
)

// termos mapeia palavra digitada -> categoria. As chaves ja estao normalizadas
// (minusculas, sem acento), porque a entrada passa por normalizar() antes.
//
// ponytail: lookup exato palavra a palavra. Nao ha stemming, plural nem
// correcao de digitacao. Se na pratica voce errar categoria com frequencia, o
// proximo passo e distancia de edicao sobre estas chaves -- nao um LLM, que
// custaria latencia e dinheiro por lancamento (RFC-0001).
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

// semAcento troca os acentos do portugues pela letra base. Resolve o caso de
// "almoço" e "almoco" precisarem cair na mesma categoria.
//
// ponytail: um Replacer da stdlib em vez de golang.org/x/text/unicode/norm.
// Sao 20 pares que cobrem o portugues inteiro; a dependencia externa so se
// pagaria se precisassemos normalizar idiomas que nao controlamos.
var semAcento = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

// normalizar deixa o texto comparavel com as chaves de termos.
func normalizar(s string) string {
	return semAcento.Replace(strings.ToLower(s))
}

// classificar devolve a categoria da primeira palavra reconhecida da entrada.
// Nenhuma palavra reconhecida devolve Outros, nunca erro.
func classificar(entrada string) Categoria {
	for _, palavra := range strings.Fields(normalizar(entrada)) {
		if c, ok := termos[palavra]; ok {
			return c
		}
	}
	return Outros
}

// receitas sao as categorias que representam dinheiro entrando.
var receitas = map[Categoria]bool{
	Salario:       true,
	Freelancer:    true,
	Investimentos: true,
}

// tipoDe deriva receita ou despesa da categoria, para que o usuario nao precise
// declarar: quem digita "salario 3500" ja disse tudo o que era preciso.
//
// Outros cai em Despesa. E ambigua por definicao (existe nas duas listas), e a
// esmagadora maioria dos lancamentos e saida de dinheiro -- o padrao que erra
// menos vezes.
func tipoDe(c Categoria) Tipo {
	if receitas[c] {
		return Receita
	}
	return Despesa
}
