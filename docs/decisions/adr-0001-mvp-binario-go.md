# ADR-0001: MVP como binário Go + página HTML + arquivo JSON

**Status:** Aceito
**Data:** 25 de setembro de 2026
**Decisor:** Augusto
**Desvia de:** [RFC-0001](rfc-0001-escolha-da-stack.md), seções 3.1 (Flutter) e 3.3 (PostgreSQL)

## Contexto

O domínio (parser, fatura, resumo) está pronto e 100% coberto desde julho, mas
ninguém registrou um gasto de verdade ainda. A tese do produto — atrito mata
hábito — continua sem teste com usuário real. O objetivo agora é ter o MVP em uso
o mais rápido possível.

O caminho do RFC-0001 exige instalar o SDK do Flutter (~3GB, sem pacote winget),
modelar persistência com Drift e, para sync, subir API + Postgres + Supabase.
São semanas até o primeiro lançamento real.

## Decisão

Para o MVP:

- **Um binário Go** em `api/cmd/app`, que reusa `parser`, `fatura` e `resumo`
  sem alteração.
- **Uma página HTML** embutida no binário (`embed`), JS puro, sem framework.
  Instalável na tela inicial do celular pelo próprio navegador.
- **Persistência em um arquivo JSON**, com escrita atômica (arquivo temporário
  + rename) para que uma queda no meio da gravação não corrompa os dados.
- **Um cartão só**, com dia de fechamento vindo de variável de ambiente.

Nenhuma dependência externa: `go.sum` continua vazio.

## Consequências

- Roda no PC; o celular acessa pela rede local. **Sem o PC ligado, não há app.**
- Acesso pela rede exige senha (Basic Auth nativo do navegador). Sem TLS, a senha
  trafega em claro na rede local: aceitável na rede de casa, **inaceitável** em
  rede pública ou exposto à internet.
- Um usuário só. Não há multiusuário, sync nem backup automático — o backup é
  copiar o arquivo JSON.

## Gatilhos para voltar ao RFC-0001

Qualquer um destes reabre a decisão:

1. Precisar usar o app com o PC desligado ou fora de casa → app nativo/Flutter.
2. Mais de um usuário → API com auth de verdade + Postgres.
3. Arquivo JSON acima de ~5MB ou lentidão perceptível → SQLite.
4. Mais de um cartão com fechamentos diferentes → entidade `Cartao` persistida.
