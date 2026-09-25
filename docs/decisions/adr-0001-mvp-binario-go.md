# ADR-0001: MVP as a Go binary + HTML page + JSON file

**Status:** Accepted
**Date:** September 25, 2026
**Decided by:** Augusto
**Deviates from:** [RFC-0001](rfc-0001-escolha-da-stack.md), sections 3.1 (Flutter) and 3.3 (PostgreSQL)

## Context

The domain (parser, statements, monthly summary) had been done and fully covered
by tests since July, but nobody had recorded a real expense with it yet. The
product thesis -- friction kills habit -- was still untested with a real user.
The goal now is to have the MVP in use as fast as possible.

The RFC-0001 path requires installing the Flutter SDK (~3GB, no winget
package), modeling persistence with Drift and, for sync, running an API +
PostgreSQL + Supabase. That is weeks before the first real entry.

## Decision

For the MVP:

- **One Go binary** in `api/cmd/app`, reusing `parser`, `fatura` and `resumo`
  unchanged.
- **One HTML page** embedded in the binary (`embed`), plain JavaScript, no
  framework. It can be installed on the phone's home screen from the browser.
- **Persistence in one JSON file**, with atomic writes (temporary file +
  rename) so a crash mid-write cannot corrupt the data.
- **A single credit card**, with its closing day from an environment variable.

No external dependencies: `go.sum` stays empty.

## Consequences

- It runs on the PC; the phone reaches it over the local network. **With the PC
  off, there is no app.**
- Network access requires a password (the browser's native Basic Auth). Without
  TLS the password travels in clear text on the local network: acceptable on a
  home network, **unacceptable** on a public network or exposed to the internet.
- A single user. No multi-user support, sync or automatic backup -- backing up
  means copying the JSON file.

## Triggers for returning to RFC-0001

Any of these reopens the decision:

1. Needing the app with the PC off or away from home → native app / Flutter, or
   a hosted server.
2. More than one user → an API with real authentication + PostgreSQL.
3. JSON file above ~5MB or noticeable slowness → SQLite.
4. More than one credit card with different closing days → a persisted `Cartao`
   (card) entity.
