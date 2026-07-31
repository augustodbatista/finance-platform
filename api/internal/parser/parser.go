// Package parser converte a entrada em linguagem natural do usuario
// ("120 mercado") em um lancamento estruturado.
//
// E dominio puro: sem I/O, sem banco, sem rede. A tese do produto e que atrito
// mata habito, entao este pacote e o caminho mais curto entre o que o usuario
// digita e um lancamento salvo.
package parser

import "time"

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
	// Data e a data da compra, nao a do registro: quem lanca "ontem" quer que
	// o gasto conte no dia em que aconteceu.
	Data time.Time
	// Forma fica vazia quando o usuario nao disse. Ver FormaNaoInformada.
	Forma FormaPagamento
}

// Parse converte a entrada do usuario em um Lancamento.
//
// O relogio entra como parametro em vez de sair de time.Now() aqui dentro.
// Sem isso a funcao deixaria de ser pura e "ontem" mudaria de resultado
// conforme o dia em que rodasse -- inclusive nos testes.
//
// A entrada e string livre vinda do usuario, entao o teto de tamanho e checado
// antes de qualquer trabalho: e a fronteira de confianca do dominio.
//
// A ordem importa. Os tokens que contem digitos mas nao sao dinheiro saem da
// string ANTES da extracao do valor, senao a regra "vale o ultimo numero"
// pegaria o pedaco errado: "mercado 120 15/03" viraria R$ 0,03.
func Parse(entrada string, agora time.Time) (Lancamento, error) {
	if len(entrada) > MaxEntrada {
		return Lancamento{}, ErrEntradaLonga
	}

	// Normalizar uma vez, no inicio: daqui para baixo todo mundo trabalha
	// sobre a mesma string, sem caixa nem acento para atrapalhar.
	data, resto, err := extrairData(normalizar(entrada), agora)
	if err != nil {
		return Lancamento{}, err
	}

	centavos, err := extrairCentavos(resto)
	if err != nil {
		return Lancamento{}, err
	}

	categoria := classificar(resto)

	return Lancamento{
		Centavos:  centavos,
		Categoria: categoria,
		Tipo:      tipoDe(categoria),
		Data:      data,
		Forma:     formaDe(resto),
	}, nil
}
