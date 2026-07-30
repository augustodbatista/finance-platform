// Package parser converte a entrada em linguagem natural do usuario
// ("120 mercado") em um lancamento estruturado.
//
// E dominio puro: sem I/O, sem banco, sem rede. A tese do produto e que atrito
// mata habito, entao este pacote e o caminho mais curto entre o que o usuario
// digita e um lancamento salvo.
package parser

// Tipo distingue entrada de saida de dinheiro.
type Tipo string

const (
	Despesa Tipo = "despesa"
	Receita Tipo = "receita"
)

// Lancamento e o resultado estruturado de uma entrada do usuario.
//
// Centavos e int64 de proposito: dinheiro em float64 corrompe saldo
// silenciosamente (0.1 + 0.2 != 0.3). A formatacao para exibicao acontece na
// borda de apresentacao, nunca aqui.
type Lancamento struct {
	Centavos  int64
	Categoria Categoria
	Tipo      Tipo
}

// Parse converte a entrada do usuario em um Lancamento.
//
// A entrada e string livre vinda do usuario, entao o teto de tamanho e checado
// antes de qualquer trabalho: e a fronteira de confianca do dominio.
func Parse(entrada string) (Lancamento, error) {
	if len(entrada) > MaxEntrada {
		return Lancamento{}, ErrEntradaLonga
	}

	centavos, err := extrairCentavos(entrada)
	if err != nil {
		return Lancamento{}, err
	}

	categoria := classificar(entrada)

	return Lancamento{
		Centavos:  centavos,
		Categoria: categoria,
		Tipo:      tipoDe(categoria),
	}, nil
}
