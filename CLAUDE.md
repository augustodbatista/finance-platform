# CLAUDE.md

> Instructions for the AI pair programmer (Claude Code) working on this repository.
> Read this file at the start of every session. It is a living document: every
> non-obvious discovery goes into **Common Hurdles** *before* moving on.

## Overview and Goal

Finance Platform: cross-platform personal finance tracking. The core thesis is that
**friction kills habit** — if logging a transaction takes more than ~20 seconds,
people stop doing it. Less friction → more habit → real control over money.
Personal MVP, designed to grow into a commercial product.

In practice, every feature has to answer: "does this increase or decrease the time
until the user logs an expense?" If it increases it, it needs a strong reason.

## Working Discipline (Extreme Programming)

Applies to every session, including future ones.

- **Roles:** Augusto brings the *what* and the *why* (direction, architecture,
  domain, priorities) and is the final authority and code reviewer. Claude brings
  the *how* (implementation, tests, proposals). Claude questions decisions, points
  out risks and proposes the simplest path — it does not just execute. A decision
  by Augusto is **not** permission to skip quality steps: if a request breaks these
  rules, say so first.
- **Security and edge cases** are Claude's responsibility to raise, even (and
  especially) when nobody asked.
- **TDD from commit 1:** red → green → refactor. No "I'll test it later".
- **Green CI is non-negotiable:** every commit on `main` is production-ready.
- **Small releases:** each commit is small, tested and potentially shippable.
- **Continuous refactoring:** pruning is part of the flow, not a separate phase.
- **Security as a habit:** handled where the attack surface appears, not in a
  security sprint at the end.

### Resolving the TDD × green CI tension

The red → green → refactor cycle happens **in the working tree**, not in history.
Committing a failing test would break "every commit is production-ready". So each
commit contains the test and the implementation together, green. The "red" stays in
the terminal.

### Test metric

We do not chase a test-to-code line ratio — it rewards filler tests. The commitment
is: **every relevant branch and edge case covered**, and uncovered code is declared
explicitly in review.

## Language

- **English:** comments, internal error strings, test names and failure messages,
  documentation, new commit messages.
- **Portuguese (pt-BR):** everything the end user or operator reads — the UI, API
  error messages, configuration errors, startup logs. The product is Brazilian.
- **Identifiers stay in Portuguese** as the domain's ubiquitous language
  (`Lancamento`, `fatura`, `parcela`, `competencia`). The README has a glossary.
- Test *inputs* are Portuguese user text (`"mercado 120 15/03"`) because that is
  what the parser parses. Never translate them.
- Commits before September 2026 are in Portuguese; history is not rewritten.

## Technology Stack

Defined in [RFC-0001](docs/decisions/rfc-0001-escolha-da-stack.md) (approved).
Do not change it without a new RFC or ADR.

**Current deviation for the MVP:** [ADR-0001](docs/decisions/adr-0001-mvp-binario-go.md) —
Go binary + HTML page + JSON file, no Flutter and no PostgreSQL. The ADR lists the
triggers that reopen the decision.

| Layer | Choice |
|---|---|
| Frontend | Flutter (Riverpod = state, GoRouter = navigation) |
| Backend | Go |
| Database | PostgreSQL (GORM for the MVP; SQLC when queries need tuning) |
| Offline | SQLite via Drift |
| Auth / Storage | Supabase |
| Parsing | Regex/rules — **not** an LLM (latency and cost per entry) |
| Container | Docker |
| CI/CD | GitHub Actions |
| Deploy | Railway or Render + Supabase |

## Directory Structure

```
/api/cmd/app            → MVP binary (main + config)
/api/internal/parser    → text → entry
/api/internal/fatura    → statements and installments
/api/internal/resumo    → dashboard numbers
/api/internal/armazem   → JSON persistence
/api/internal/web       → HTTP + page (static/ embedded in the binary)
/docs/decisions         → RFCs and ADRs
.github/workflows/ci.yml
.golangci.yml
osv-scanner.toml
```

`app/` (Flutter) does not exist — postponed by ADR-0001. It is created in the commit
that brings its first code.

## Environment Variables

Rule: every new variable goes into this table **in the same commit that introduces
it**, with name, purpose, required/optional and where it is read. Secrets never go
into the repository — `.env` is in `.gitignore` and the SAST job runs the
`p/secrets` ruleset precisely to catch accidental leaks.

| Name | Purpose | Required | Read by |
|---|---|---|---|
| `FINANCE_DIA_FECHAMENTO` | Credit card closing day (1..31) | **Yes** | `cmd/app` → `resumo` |
| `FINANCE_ENDERECO` | `host:port` to listen on. Default `127.0.0.1:8080` (this machine only) | No | `cmd/app` |
| `FINANCE_DADOS` | Path of the JSON data file. Default `dados.json` | No | `cmd/app` → `armazem` |
| `FINANCE_SENHA` | Basic Auth password, at least 8 characters | **Yes off loopback** | `cmd/app` → `web` |

`:8080` with no host listens on **every** interface — it looks local, it is not, and
it requires a password. Tested.

## Main Packages and Models

### `api/internal/parser` — pure domain, no I/O

`Parse(entrada string, agora time.Time) (Lancamento, error)` turns the user's
one-line input into a structured entry. 100% coverage.

```go
type Lancamento struct {
    Centavos  int64          // purchase TOTAL, never float64
    Categoria Categoria      // closed set
    Tipo      Tipo           // despesa | receita (expense | income), from the category
    Data      time.Time      // purchase date, truncated to midnight
    Forma     FormaPagamento // "" when the user did not say
    Parcelas  int            // 1 for a single payment
}
```

`Parse` takes the clock as a parameter. Never call `time.Now()` inside the domain:
it breaks purity and makes the `"ontem"` (yesterday) test depend on the calendar.

| File | Responsibility |
|---|---|
| `parser.go` | `Parse`, `Lancamento`, `Tipo`, size limit, **pipeline order** |
| `valor.go` | pt-BR amount → cents; range errors; `MaxEntrada` |
| `categoria.go` | closed set, term map, `normalizar`, `tipoDe` |
| `data.go` | `hoje`/`ontem`/`dd/mm[/yy[yy]]`; `ErrDataInvalida` |
| `pagamento.go` | `FormaPagamento` and its term map |
| `parcela.go` | `Nx` token; delegates ceiling and error to `fatura` |

**The pipeline order in `Parse` is a rule, not style.** Tokens that contain digits
but are not money — dates and installments — are removed from the string **before**
the amount is extracted. Otherwise the "last number wins" rule picks the wrong piece
and the app stores a wrong amount **without failing and without warning**:

| Input | Without removal | Correct |
|---|---|---|
| `"mercado 120 15/03"` | R$ 0,03 | R$ 120,00 |
| `"mercado 120 15/03/2026"` | R$ 20,26 | R$ 120,00 |
| `"300 mercado 3x"` | R$ 3,00 | R$ 300,00 in 3 installments |

Every new token that contains a digit joins this removal queue, with a test.

Normalization (`normalizar`) runs **once**, at the start of `Parse`. The rest of the
pipeline expects the already-normalized string.

Exported errors: `ErrSemValor`, `ErrValorInvalido`, `ErrValorNaoPositivo`,
`ErrEntradaLonga`, `ErrDataInvalida`. Compare with `errors.Is`, never by string.

Behaviors that are decisions, not accidents — all tested:

- Unknown term → `Outros` (other), never an error.
- `Outros` → `Despesa` (ambiguous; expenses are the overwhelming majority).
- `"cartao"` (card) alone → `Credito` (people paying by debit usually say "débito").
- No payment method → `FormaNaoInformada`. The parser reports what it found;
  applying the user's default belongs to the layer that knows user settings.
- `Parcelas > 1` infers `Credito` **only when no method was given** — inference
  fills a gap, it does not override the user.
- Negative sign ignored: `-5 mercado` is an expense of R$ 5,00.
- More than two decimals truncates, never rounds: `42,555` → R$ 42,55.
- `dd/mm` with no year → most recent past occurrence (logging late is routine;
  logging in the future is almost always a mistake).

Not there yet: account and description on `Lancamento` — they arrive with the slice
that needs them.

### `api/internal/fatura` — pure domain, no I/O

Calendar math and splitting money are not text analysis. Works only with primitives
and **does not import `parser`** (the dependency is `parser → fatura`). 100% coverage.

```go
type Competencia struct { Ano int; Mes time.Month }  // no day, on purpose
func De(compra time.Time, diaFechamento int) (Competencia, error)
func Dividir(total int64, parcelas int, compra time.Time, diaFechamento int) ([]Parcela, error)
```

- **A purchase on the closing day itself goes to the next statement.** Augusto's
  decision (2026-07-30); it varies by issuer, so check it against a real statement.
- `Competencia` has no day because a statement is a monthly bucket — adding months
  to a (year, month) pair removes the "January 31 + 1 month" problem for free.
- The comparison is by day number, with no clamping to the month length. A card
  closing on the 31st handles itself: in February every day is < 31.
- The division remainder goes on the **first** installment. Installments always add
  up to the total — tested as an invariant, not as an example.
- `MaxParcelas` and `ErrParcelasInvalidas` live **here**, not in the parser: this is
  where the N-sized slice is allocated, and how many installments a card accepts is
  a card rule, not a text rule.

The closing day is a parameter, not a `Cartao` entity — without persistence there
would be nowhere to store it. The entity arrives with the database.

**Consequence for the dashboard:** "expenses this month" means "installments whose
statement is this month", not "purchases made this month". Two different numbers.

### `api/internal/resumo` — pure domain, no I/O

`Mensal(mes, lancamentos, diaFechamento) (Resumo, error)` produces the dashboard
numbers. Depends on `parser` and `fatura`; nothing depends on it. 100% coverage.

The rule that justifies the package is not the sum, it is the distinction between
two numbers that look the same:

| Entry | Counts in |
|---|---|
| Income | month of `Data` — ignores statements and installments (a card is for spending, not receiving) |
| Non-credit expense (debit, pix, cash, **not stated**) | month of `Data` |
| Credit card expense | statement of **each installment** |

`FormaNaoInformada` counts in the month of the date: the parser does not invent a
method, and treating the unknown as credit would postpone money that may already
have left the account.

Savings (`Economia`) go negative in a month that only has a statement to pay — that
is information, not an error.

**There is no current balance yet.** It needs an opening balance and statement
payments modeled as entries; neither exists. It arrives with the slice that brings
accounts and statement payments.

**Known debt:** `Lancamento` lives in `parser`, so `resumo`, `armazem` and `web`
import the text package just for the type. The trigger recorded for moving it into
its own domain package ("a third consumer appears") has fired; it was deferred to
ship the MVP. Do it in the next slice that touches `Lancamento`.

### `api/internal/armazem` — JSON file persistence

Stores entries in a local JSON file ([ADR-0001](docs/decisions/adr-0001-mvp-binario-go.md)).
`Abrir` (open), `Adicionar` (add), `Listar` (list, newest first, returns a copy),
`Remover` (remove). Safe for concurrent use. 90.4% coverage.

Guarantees, all tested:

- **Atomic writes:** temporary file in the same directory + `rename`. A crash
  mid-write leaves the old file whole, never half-written.
- **Memory only changes after the disk confirms.** If a write fails, memory and disk
  stay equal — the screen never shows an entry that disappears on restart.
- **A corrupted file is an error, never "start empty"**: the next `Adicionar` would
  overwrite everything. The file is left untouched for recovery.
- **IDs are never reused** (`proximo_id` is persisted): a delayed DELETE cannot
  remove the wrong record.
- Permission `0600`: financial data, only the owner reads it.
- Stores the **original typed text** — the best description there is.

Uncovered and declared: failures when encoding, writing, syncing, closing and
chmod-ing the temp file. They cannot be induced without a filesystem mock, and that
mock would be an interface with a single implementation.

### `api/internal/web` — HTTP and the MVP page

`Novo(Config) http.Handler`. Routes: `GET /` (page), `GET/POST /api/lancamentos`,
`DELETE /api/lancamentos/{id}`, `GET /api/resumo?mes=YYYY-MM`. The page (`static/`)
is embedded in the binary. 98.9% coverage.

Attack surfaces and their handling, all tested:

| Threat | Handling |
|---|---|
| Anyone on the same network | Basic Auth (native prompt). SHA-256 + constant-time comparison: not even the password length leaks |
| CSRF (the browser resends the password on its own) | POST requires `Content-Type: application/json` → 415 otherwise. A form on another site cannot send that without a CORS preflight, which the server never grants |
| XSS through typed text | Front end uses `textContent`, never `innerHTML`; CSP `default-src 'self'` (nothing inline — that is why CSS and JS are separate files); `frame-ancestors 'none'` |
| Oversized bodies | 4KB `MaxBytesReader` → 413 |
| Slow clients holding connections | Explicit timeouts on `http.Server` |

Domain errors become 400 with a Portuguese message that says how to fix the input.
A disk failure becomes 500 saying **what did not happen** ("Nada foi gravado" —
nothing was saved) so the user knows to retype. Uncovered and declared: the
`default` branch of `mensagem`, unreachable while the parser only returns errors
that are already mapped.

No TLS: the password travels in clear text on the local network. Acceptable at
home, unacceptable on a public network (ADR-0001).

## Design Patterns and Conventions

- **Money is `int64` cents. Never `float64`.** `0.1 + 0.2 != 0.3` in floating point;
  a balance stored as float corrupts silently. Formatting for display only happens at
  the presentation edge.
- **User input is pt-BR:** the comma is the decimal separator (`42,50`) and the dot
  is the thousands separator (`1.234,56`). The parser assumes this; tests cover both.
- **Categories are a closed set**, not free text.
  Income: Salário, Freelancer, Investimentos, Outros.
  Expenses: Alimentação, Mercado, Transporte, Casa, Saúde, Lazer, Educação,
  Assinaturas, Outros.
  Unrecognized input falls into "Outros" and is not an error — zero friction matters
  more than category precision.
- **Input with more than one number: the last one wins.** `"2 cafés 15"` → R$ 15,00;
  `"3x uber 18"` → R$ 18,00. Augusto's product decision (2026-07-30). A one-line rule,
  consistent with every single-number example (`120 mercado`, `Uber 18`,
  `Salário 3500`), that **never rejects and never asks** — rejecting ambiguous input
  would add friction exactly where the product thesis says friction kills habit.
- **Pure domain at the center:** parser and business rules have no I/O, no database,
  no HTTP. Testable with `go test` and no infrastructure.
- **External integrations behind interfaces** (RFC-0001's mitigation for the Go
  ecosystem and Supabase lock-in). Internal IDs are our own UUIDs mapped to the
  Supabase ID — never couple business rules to the provider's ID.
- **AI decoupled:** if an LLM provider comes in, it goes behind an interface, with
  business rules outside the model.

## Main System Flow

```
User input ("120 mercado")
  → Parser (regex + rules, pure domain)
  → Validated entry (amount in cents, category, account, date)
  → Local persistence (SQLite/Drift, offline-first)
  → Sync → Go API → PostgreSQL
  → Dashboard (balance, income this month, expenses this month, savings)
```

**MVP (ADR-0001):** input → `parser` → `armazem` (JSON) → `resumo` → page. No sync,
no PostgreSQL, no Flutter. The flow above is still the target; the ADR lists the
triggers for going back to it.

## Security — known surfaces

| Surface | Where it appears | Handling |
|---|---|---|
| Untrusted input | The parser receives free text from the user | Input size limit; amount range validation (> 0, no `int64` overflow) |
| ReDoS | Regexes over user input | Go's `regexp` is RE2 (linear, no backtracking) — immune by construction. **Do not** swap it for a backtracking library |
| Secret leaks | Supabase keys | `.env` in `.gitignore` + `p/secrets` ruleset in SAST |
| Network exposure | MVP server | Password required off loopback; CSRF, XSS, body size and timeouts handled in `web` |
| SQL injection | Future persistence layer | GORM parameterizes; never concatenate SQL |
| AuthZ | Future multi-user API | Every query filters by user on the server; never trust an ID from the client |

Rate limiting has no surface while the server stays on the local network; it becomes
mandatory the moment the app is exposed to the internet (password brute force).

## Vulnerability triage

"Always green" needs a release valve, or it becomes theater. When `osv-scanner` or
`govulncheck` report something:

1. Is there a patch? Update the dependency. Done.
2. No patch, but `govulncheck` says the code **does not reach** the vulnerable
   function? Record it in `osv-scanner.toml` with `reason` and `ignoreUntil` (at most
   90 days).
3. No patch and reachable? Isolate/mitigate in code or replace the dependency.

Ignoring without a `reason` and a review date is a violation of the green-CI rule,
not an exception to it.

## Common Hurdles

Every new gotcha goes here **before** moving on.

- **Flutter is not installed on this machine** (Go and Docker are). Before any
  Flutter work, confirm `flutter --version` in a fresh terminal — PATH does not
  reload in a terminal that is already open.
- **Flutter has no official winget package.** Only a standalone `Google.DartSDK`
  exists. Install from the zip + PATH, in a path without spaces and outside
  `Program Files` — the installer fails in both.
- **`make` does not exist on this machine.** No Makefile; raw commands are
  documented here.
- **`go` is not on the agent shell's PATH.** The binary lives in
  `C:\Program Files\Go\bin`. Export it before any Go command:
  `export PATH="$PATH:/c/Program Files/Go/bin"`.
- **`go test -race` does not run on this machine.** `-race` needs cgo, cgo needs a C
  compiler, and there is no `gcc` here. **Locally** use `go test -cover ./...`; CI
  keeps `-race` because the Linux runner has a C toolchain. Do not install MinGW just
  for this.
- **`core.autocrlf=true` in this machine's global git config.** Without
  `.gitattributes`, files would be CRLF in the local tree and LF on the Linux runner,
  making `gofmt` and `dart format` disagree between this machine and CI — green
  locally, red in CI, with no visible difference in the diff. Fixed by
  `* text=auto eol=lf` in `.gitattributes`. **Do not** run `git config core.autocrlf`
  to "fix" anything: `.gitattributes` takes precedence and is versioned; the global
  config is not.
- **A Go module with only `go.mod` and zero `.go` files fails CI.** `golangci-lint`
  aborts with "no go files to analyze". So `go.mod` never goes into an
  infrastructure-only commit: it arrives with the first `.go` file and its test.
- **`google/osv-scanner-action` does not publish a floating major tag.** There is no
  `v2`; only full releases (`v2.3.8`). The pin must be exact — "simplifying" to `@v2`
  breaks the job with *"unable to find version"*. It is the only action in CI like
  this; all the others (`actions/checkout@v7`, `dorny/paths-filter@v4`,
  `actions/setup-go@v7`, `golangci/golangci-lint-action@v9`,
  `golang/govulncheck-action@v1`, `subosito/flutter-action@v2`) have major tags.
- **`p/dart` and `p/flutter` do not exist in the semgrep registry** (HTTP 404), and
  neither does `p/go` — the right one is `p/golang`. One invalid config fails the
  whole scan with exit 7; it is not ignored. Dart static analysis is handled by
  `flutter analyze`.
- **Actions on Node 20 already emit deprecation warnings** on the runner. Nothing
  breaks today; it will break on its own later. Keep pins on current majors.
- **`golangci-lint` v2 changed its config format** (it requires `version: "2"`) and
  needs `golangci-lint-action@v8` or later (we use `@v9`). A config parse error on the
  first run means the action/config pair is mismatched — fix both together.
- **The `go` directive in `go.mod` pins the standard library version, and the
  standard library has CVEs.** With no commit of ours, `main` turned red in Aug/2026:
  8 standard library vulnerabilities fixed in 1.26.6 were published while `go.mod`
  said `1.26.5`. The `go` job stayed green (`govulncheck` saw that the domain does not
  reach the affected functions); `osv-scanner`, which does no such analysis for the
  standard library, failed. Fix: bump the directive to the **latest patch of the same
  minor** (`go mod edit -go=1.26.X`) — the toolchain downloads itself
  (`GOTOOLCHAIN=auto`). Moving to a new minor (1.27) is a separate decision and is not
  what fixes this.
- **Right after `git push`, `gh run list --limit 1` still shows the previous run.**
  The new run takes a few seconds to register; reading results in that window makes
  an old run look current (it once led to diagnosing a failure that had already been
  fixed). Filter by commit: `gh run list --commit $(git rev-parse HEAD)`.
- **The fields of `parser.Lancamento` are a file format.** `armazem` serializes the
  struct without JSON tags, so renaming a field (`Centavos` → `Valor`) makes the
  existing file load with that field zeroed — **with no error**. Renaming requires a
  `json:"old_name"` tag or a file migration.
- **Run the linter locally before pushing**, because errcheck/gosec fail things that
  `go vet` lets through (it already caught an unchecked `defer os.Remove` and
  `Close`): `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run ./...`
  (in `api/`; the first run downloads the linter's dependencies, not the project's).
- **The Flutter CI jobs stay dormant until `app/` exists** (`paths-filter` skips
  them). That is deliberate: the first commit in `app/` will probably surface a config
  problem, and fixing it is part of that commit.
- **Bash heredocs in the agent shell sometimes break on quotes** ("unexpected EOF
  while looking for matching `''"), especially Python or JSON with backslashes. Write
  the script or file with the editor tool and run it, instead of inlining it.

## Local commands

From `api/`, with PATH already exported:

```bash
export PATH="$PATH:/c/Program Files/Go/bin" && cd /c/finance-platform/api && gofmt -l . && go test -cover ./...
```

Run the MVP on this machine only:

```bash
cd /c/finance-platform/api && FINANCE_DIA_FECHAMENTO=28 go run ./cmd/app
```

To open it on the phone (same wifi), listen on every interface with a password —
the log prints the address to type on the phone:

```bash
cd /c/finance-platform/api && FINANCE_DIA_FECHAMENTO=28 FINANCE_ENDERECO=0.0.0.0:8080 FINANCE_SENHA=change-this-password go run ./cmd/app
```

The first time, Windows asks whether to allow the program through the firewall:
allow it on **private networks** only.

No local `-race` (see Common Hurdles). CI runs `-race` on Linux.

## Definition of Done (post-implementation checklist)

- [ ] Test written before the code, covering the happy path and edge cases
- [ ] `go test -race ./...` (in `api/`) and `flutter test` (in `app/`) passing
- [ ] `gofmt -l .` empty; `golangci-lint run` clean (includes `gosec`)
- [ ] `dart format --set-exit-if-changed .` and `flutter analyze` clean
- [ ] `govulncheck ./...` and `osv-scanner` with no new findings — or the finding
      recorded in `osv-scanner.toml` with a reason and review date
- [ ] `semgrep` with no new findings
- [ ] No new environment variable without a row in the table above
- [ ] Common Hurdles updated if something non-obvious came up
- [ ] Small, cohesive commit; the message explains **why**, not what
