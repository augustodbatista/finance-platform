# CLAUDE.md

> Leia este arquivo no início de cada sessão. Ele é documento vivo: toda descoberta
> não óbvia entra em Common Hurdles **antes** de seguir para o próximo passo.

## Visão Geral e Objetivo

Finance Platform: controle financeiro pessoal multiplataforma. A tese central é que
**atrito mata hábito** — se registrar uma movimentação leva mais de ~20 segundos, o
usuário para de usar. Menor atrito → mais hábito → mais controle financeiro real.
MVP pessoal, arquitetado para evoluir a produto comercial.

Consequência prática: toda feature responde "isso aumenta ou diminui o tempo até o
usuário registrar um gasto?". Se aumenta, precisa de justificativa forte.

## Disciplina de Trabalho (Extreme Programming)

Vigente em todas as sessões, incluindo futuras.

- **Papéis:** Augusto traz o *quê* e o *porquê* (direção, arquitetura, domínio,
  prioridades) e é a autoridade final e o code review. Claude traz o *como*
  (implementação, testes, propostas). Claude questiona, aponta riscos e propõe o
  caminho mais simples — não apenas executa. Decisão do Augusto **não** é autorização
  para pular etapas de qualidade: se um pedido quebra estas regras, avisar antes.
- **Segurança e casos de borda** são responsabilidade do Claude levantar, mesmo (e
  principalmente) quando não são pedidos explicitamente.
- **TDD desde o commit 1:** red → green → refactor. Nenhuma feature "testo depois".
- **CI verde é inviolável:** todo commit em `main` é production-ready.
- **Small releases:** cada commit é pequeno, testado e potencialmente entregável.
- **Refactoring contínuo:** podar é parte do fluxo, não uma fase separada.
- **Segurança como hábito:** tratada onde a superfície de risco aparece, não numa
  sprint de segurança no fim.

### Resolvendo a tensão TDD × CI verde

O ciclo red → green → refactor acontece **na árvore de trabalho**, não no histórico.
Commitar um teste vermelho violaria "todo commit é production-ready". Portanto: o
commit contém teste + implementação juntos, verde. O "red" fica no seu terminal.

### Métrica de teste

Não perseguimos razão de linhas teste/produção — isso premia teste de enchimento. O
compromisso é: **todo branch e caso de borda relevante coberto**, e trechos
descobertos são declarados explicitamente na revisão.

## Stack Tecnológico

Definida em [RFC-0001](docs/decisions/rfc-0001-escolha-da-stack.md) (aprovado).
Não alterar sem novo RFC.

| Camada | Escolha |
|---|---|
| Frontend | Flutter (Riverpod = estado, GoRouter = navegação) |
| Backend | Go |
| Banco | PostgreSQL (GORM no MVP; SQLC quando exigir queries otimizadas) |
| Offline | SQLite via Drift |
| Auth / Storage | Supabase |
| Parsing | Regex/regras — **não** LLM (custo e latência por lançamento) |
| Container | Docker |
| CI/CD | GitHub Actions |
| Deploy | Railway ou Render + Supabase |

## Estrutura de Diretórios

```
/api                    → backend Go
/app                    → Flutter (mobile, web, desktop)
/docs/decisions         → RFCs e ADRs
.github/workflows/ci.yml
.golangci.yml
osv-scanner.toml
```

`api/` e `app/` ainda não existem — nascem no commit que trouxer o primeiro código de
cada um. Git não versiona diretório vazio, e criar scaffolding "para depois" é dívida
sem contrapartida.

## Variáveis de Ambiente

Nenhuma ainda — o ciclo 1 é domínio puro, sem I/O.

Regra: cada env var nova entra nesta tabela **no mesmo PR que a introduz**, com nome,
propósito, obrigatória/opcional e onde é lida. Segredo nunca vai para o repositório —
`.env` está no `.gitignore` e o job de SAST roda a ruleset `p/secrets` justamente para
pegar vazamento acidental.

| Nome | Propósito | Obrigatória | Onde é usada |
|---|---|---|---|
| — | — | — | — |

## Principais Serviços, Jobs e Models

Vazio — nada implementado. Preenchido a cada slice vertical.

Primeiro model previsto: `Movimentacao` (valor em centavos, tipo, categoria, conta,
data, descrição).

## Design Patterns e Convenções

- **Dinheiro é `int64` em centavos. Nunca `float64`.** `0.1 + 0.2 != 0.3` em ponto
  flutuante; um saldo financeiro em float corrompe silenciosamente. Formatação para
  exibição acontece só na borda de apresentação.
- **Locale é pt-BR na entrada do usuário:** vírgula é separador decimal (`42,50`),
  ponto é separador de milhar (`1.234,56`). O parser assume isso; testes cobrem ambos.
- **Categorias são um conjunto fechado**, não texto livre.
  Receitas: Salário, Freelancer, Investimentos, Outros.
  Despesas: Alimentação, Mercado, Transporte, Casa, Saúde, Lazer, Educação,
  Assinaturas, Outros.
  Entrada não reconhecida cai em "Outros", não gera erro — atrito zero é mais
  importante que precisão de categoria.
- **Domínio puro no centro:** parser e regras de negócio sem I/O, sem banco, sem HTTP.
  Testáveis com `go test` sem infraestrutura.
- **Integrações externas atrás de interfaces** (mitigação do RFC-0001 para o
  ecossistema Go e para o lock-in do Supabase). ID interno em UUID próprio, mapeado
  para o ID do Supabase — nunca acoplar regra de negócio ao ID do provedor.
- **IA desacoplada:** se um provedor de LLM entrar, é atrás de interface, com regra de
  negócio fora do modelo.

## Fluxo Principal do Sistema

```
Entrada do usuário ("120 mercado")
  → Parser (regex + regras, domínio puro)
  → Movimentacao validada (valor em centavos, categoria, conta, data)
  → Persistência local (SQLite/Drift, offline-first)
  → Sync → API Go → PostgreSQL
  → Dashboard (saldo, receitas do mês, despesas do mês, economia)
```

Hoje só a primeira seta existe como escopo. As demais entram slice a slice.

## Segurança — superfícies conhecidas

| Superfície | Onde aparece | Tratamento |
|---|---|---|
| Entrada não confiável | Parser recebe string livre do usuário | Limite de tamanho da entrada; validação de faixa do valor (> 0, sem overflow de `int64`) |
| ReDoS | Regex sobre entrada do usuário | O `regexp` do Go é RE2 (linear, sem backtracking) — imune por construção. **Não** trocar por biblioteca com backtracking |
| Vazamento de segredo | Chaves Supabase | `.env` no `.gitignore` + ruleset `p/secrets` no SAST |
| Injeção SQL | Futuro, camada de persistência | GORM parametriza; nunca concatenar SQL |
| AuthZ | Futuro, API multiusuário | Toda query filtra por usuário no servidor, nunca confia em ID vindo do cliente |

Rate limiting, SSRF e path traversal ainda não têm superfície — entram quando a API
HTTP existir.

## Triagem de vulnerabilidades

O "verde inviolável" precisa de válvula, senão vira teatro. Quando `osv-scanner` ou
`govulncheck` apontarem algo:

1. Existe patch? Atualizar a dependência. Fim.
2. Sem patch, mas `govulncheck` diz que o código **não alcança** a função vulnerável?
   Registrar em `osv-scanner.toml` com `reason` e `ignoreUntil` (máx. 90 dias).
3. Sem patch e alcançável? Isolar/mitigar no código ou trocar a dependência.

Ignorar sem `reason` e sem data de revisão é violação da regra de CI verde, não
exceção a ela.

## Common Hurdles

Toda pegadinha nova entra aqui **antes** de seguir.

- **Go, Flutter e Docker não vêm instalados nesta máquina.** Antes do primeiro ciclo,
  confirmar `go version`, `flutter --version` e `docker --version` em um terminal
  novo — o PATH não recarrega no terminal já aberto.
- **Flutter não tem pacote oficial no winget.** Só existe `Google.DartSDK` avulso.
  Instalação é zip + PATH, em caminho sem espaços e fora de `Program Files` — o
  instalador falha em ambos.
- **`make` não existe nesta máquina.** Nada de Makefile; comandos crus documentados
  aqui.
- **`core.autocrlf=true` no git global desta máquina.** Sem `.gitattributes`, os
  arquivos ficariam CRLF na árvore local e LF no runner Linux, fazendo `gofmt` e
  `dart format` divergirem entre a sua máquina e o CI — verde local, vermelho no CI,
  sem diferença visível no diff. Resolvido por `* text=auto eol=lf` no
  `.gitattributes`. **Não** rode `git config core.autocrlf` para "consertar" nada:
  o `.gitattributes` tem precedência e é versionado, a config global não.
- **Módulo Go só com `go.mod` e zero arquivos `.go` reprova no CI.** `golangci-lint`
  aborta com "no go files to analyze". Por isso `api/go.mod` **não** entra sozinho num
  commit de infra: nasce junto com o primeiro `.go` e seu teste.
- **Os jobs Flutter do CI ficam dormentes até `app/` existir** (o `paths-filter` os
  desliga). É deliberado, não acidente: o primeiro commit em `app/` provavelmente
  acusa problema de config, e esse ajuste faz parte daquele commit.
- **`golangci-lint` v2 mudou o formato do config** (exige `version: "2"`) e pede
  `golangci-lint-action@v8`. Se a primeira execução falhar com erro de parse, é
  incompatibilidade do par action/config — corrigir os dois juntos.

## Definição de Pronto (checklist pós-implementação)

- [ ] Teste escrito antes do código, cobrindo caminho feliz e casos de borda
- [ ] `go test -race ./...` (em `api/`) e `flutter test` (em `app/`) passando
- [ ] `gofmt -l .` vazio; `golangci-lint run` limpo (inclui `gosec`)
- [ ] `dart format --set-exit-if-changed .` e `flutter analyze` limpos
- [ ] `govulncheck ./...` e `osv-scanner` sem finding novo — ou finding registrado em
      `osv-scanner.toml` com motivo e data de revisão
- [ ] `semgrep` sem finding novo
- [ ] Nenhuma env var nova sem linha na tabela acima
- [ ] Common Hurdles atualizado se algo não óbvio apareceu
- [ ] Commit pequeno e coeso; a mensagem explica o **porquê**, não o quê
