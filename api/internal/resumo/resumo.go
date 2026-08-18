// Package resumo agrega lancamentos nos numeros que o dashboard mostra.
//
// E dominio puro: sem I/O, sem banco, sem rede.
//
// A regra que justifica o pacote existir nao e a soma, e a diferenca entre dois
// numeros que parecem o mesmo: despesa a vista pesa no dia em que aconteceu,
// despesa no credito pesa na fatura em que cai. Uma compra parcelada feita hoje
// aparece em varios meses; uma compra no debito feita hoje aparece so neste.
// Somar os dois pela data da compra e a forma mais facil de mostrar um numero
// errado com cara de certo.
package resumo

import (
	"time"

	"github.com/augustodbatista/finance-platform/api/internal/fatura"
	"github.com/augustodbatista/finance-platform/api/internal/parser"
)

// Resumo sao os numeros de um mes.
//
// ponytail: sem "Saldo atual" ainda. Saldo exige saldo inicial e o pagamento da
// fatura modelado como lancamento -- nenhum dos dois existe. Entra no slice que
// trouxer conta e pagamento de fatura.
type Resumo struct {
	ReceitasCentavos int64
	DespesasCentavos int64
	// EconomiaCentavos e receitas menos despesas. Fica negativo em mes que so
	// tem fatura para pagar, e isso e informacao, nao erro.
	EconomiaCentavos int64
}

// Mensal soma os lancamentos que pertencem a competencia informada.
//
// Receita conta no mes da data e ignora fatura e parcelas: cartao de credito e
// forma de gastar, nao de receber.
//
// Despesa no credito conta na competencia de cada parcela; nas demais formas,
// no mes da data da compra. FormaNaoInformada cai no mes da data -- o parser
// nao inventa forma, e tratar o desconhecido como credito adiaria dinheiro que
// talvez ja tenha saido.
func Mensal(mes fatura.Competencia, lancamentos []parser.Lancamento, diaFechamento int) (Resumo, error) {
	var r Resumo

	for _, l := range lancamentos {
		if l.Tipo == parser.Receita {
			if noMes(l.Data, mes) {
				r.ReceitasCentavos += l.Centavos
			}
			continue
		}

		if l.Forma != parser.Credito {
			if noMes(l.Data, mes) {
				r.DespesasCentavos += l.Centavos
			}
			continue
		}

		parcelas, err := fatura.Dividir(l.Centavos, l.Parcelas, l.Data, diaFechamento)
		if err != nil {
			return Resumo{}, err
		}
		for _, p := range parcelas {
			if p.Competencia == mes {
				r.DespesasCentavos += p.Centavos
			}
		}
	}

	r.EconomiaCentavos = r.ReceitasCentavos - r.DespesasCentavos
	return r, nil
}

// noMes diz se a data cai na competencia.
func noMes(data time.Time, mes fatura.Competencia) bool {
	return data.Year() == mes.Ano && data.Month() == mes.Mes
}
