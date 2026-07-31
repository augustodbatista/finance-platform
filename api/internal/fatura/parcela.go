package fatura

import (
	"errors"
	"time"
)

var (
	// ErrParcelasInvalidas indica numero de parcelas menor que 1 ou acima de
	// MaxParcelas.
	ErrParcelasInvalidas = errors.New("fatura: numero de parcelas invalido")
	// ErrTotalInsuficiente indica total que nao da nem um centavo por parcela.
	ErrTotalInsuficiente = errors.New("fatura: total insuficiente para o numero de parcelas")
)

// MaxParcelas e o teto de parcelas aceitas em todo o dominio.
//
// Mora aqui, e nao no parser, porque quantas parcelas um cartao aceita e regra
// de cartao, nao de texto -- e porque e aqui que o risco se materializa:
// Dividir aloca um slice de tamanho N vindo, em ultima instancia, de uma linha
// digitada pelo usuario. Sem teto, "999999999x 100" seria alocacao de bilhoes
// de itens a partir de uma frase. 99 ja passa de qualquer plano que um emissor
// brasileiro oferece.
const MaxParcelas = 99

// Parcela e um pedaco de uma compra parcelada, com a fatura em que cai.
type Parcela struct {
	Numero      int
	Centavos    int64
	Competencia Competencia
}

// Dividir reparte o total de uma compra entre parcelas, cada uma na sua fatura.
//
// O total e o valor cheio da compra: "3x 300" sao tres parcelas de R$ 100, nao
// tres de R$ 300. Quem digita informa quanto gastou e o sistema faz a conta.
//
// Divisao que nao fecha poe a sobra na primeira parcela -- convencao dos
// emissores brasileiros, entao o app bate com a fatura de verdade. A soma das
// parcelas sempre fecha com o total: nao existe centavo evaporando.
//
// A primeira parcela cai na fatura da data da compra (ver De) e cada seguinte
// avanca um mes de competencia. Como competencia nao tem dia, avancar mes aqui
// nao esbarra em mes curto.
func Dividir(totalCentavos int64, parcelas int, compra time.Time, diaFechamento int) ([]Parcela, error) {
	if parcelas < 1 || parcelas > MaxParcelas {
		return nil, ErrParcelasInvalidas
	}
	// Cobre de uma vez total zero, total negativo e total que daria parcela de
	// zero centavo -- nenhum dos tres descreve uma compra.
	if totalCentavos < int64(parcelas) {
		return nil, ErrTotalInsuficiente
	}

	primeira, err := De(compra, diaFechamento)
	if err != nil {
		return nil, err
	}

	base := totalCentavos / int64(parcelas)
	sobra := totalCentavos % int64(parcelas)

	ps := make([]Parcela, parcelas)
	for i := range ps {
		centavos := base
		if i == 0 {
			centavos += sobra
		}
		ps[i] = Parcela{
			Numero:      i + 1,
			Centavos:    centavos,
			Competencia: primeira.AdicionarMeses(i),
		}
	}
	return ps, nil
}
