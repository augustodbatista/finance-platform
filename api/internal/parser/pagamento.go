package parser

import "strings"

// FormaPagamento diz por onde o dinheiro saiu ou entrou.
//
// Importa mais do que parece: uma compra no credito nao muda o saldo hoje, so
// quando a fatura e paga. Sem esta informacao o "Saldo atual" do dashboard
// mente para quem usa cartao, que e quase todo mundo.
type FormaPagamento string

const (
	// FormaNaoInformada e o zero value: o usuario nao disse.
	//
	// O parser relata o que achou e nao inventa padrao. Quem aplica a
	// preferencia do usuario e a camada que conhece configuracao de usuario --
	// dominio puro nao deve conhecer gosto de gente.
	FormaNaoInformada FormaPagamento = ""

	Dinheiro FormaPagamento = "dinheiro"
	Pix      FormaPagamento = "pix"
	Debito   FormaPagamento = "debito"
	Credito  FormaPagamento = "credito"
)

// formas mapeia palavra digitada -> forma de pagamento. Chaves ja normalizadas.
var formas = map[string]FormaPagamento{
	"dinheiro": Dinheiro,
	"especie":  Dinheiro,
	"cash":     Dinheiro,

	"pix": Pix,

	"debito": Debito,

	"credito": Credito,
	// "cartao" sozinho e ambiguo. Credito e a leitura majoritaria, e a mesma
	// logica de Outros -> Despesa se aplica: escolher o caso mais provavel
	// custa menos que perguntar. Quem paga no debito costuma dizer "debito".
	"cartao": Credito,
}

// formaDe devolve a forma de pagamento mencionada na entrada, ou
// FormaNaoInformada. Espera a entrada ja normalizada (ver normalizar).
func formaDe(entrada string) FormaPagamento {
	for _, palavra := range strings.Fields(entrada) {
		if f, ok := formas[palavra]; ok {
			return f
		}
	}
	return FormaNaoInformada
}
