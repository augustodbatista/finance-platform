# RFC-0001: Core Technology Stack

**Status:** Approved
**Date:** July 3, 2026
**Author:** Tech Lead / Architecture
**Tags:** `architecture`, `stack`, `decisions`

> **Current deviation:** the MVP runs as a single Go binary with an HTML page and
> a JSON file instead of Flutter + PostgreSQL. See
> [ADR-0001](adr-0001-mvp-binario-go.md), which also lists the triggers for
> returning to this stack.

## 1. Context and Problem

The project needs the foundation of a robust, scalable, long-lived finance
platform. We must define the core technologies that will carry development from
the MVP to future AI integrations, with high backend performance and
cross-platform reach on the frontend, without prohibitive upfront costs.

## 2. Decision Criteria

* **MVP time-to-market:** technologies that allow fast iteration.
* **Scalability and performance:** able to handle high transaction volume and
  concurrency (a finance-oriented architecture).
* **Cross-platform reach:** least effort to ship on different operating systems.
* **Learning curve and portfolio value:** tools highly valued in the backend and
  modern software engineering market.
* **Cost pragmatism:** free or very low-cost tools for the initial phase, with a
  clear migration path.

## 3. Options Considered and Decisions

### 3.1. Frontend: Mobile, Desktop and Web
* **Options:** React Native, Kotlin Multiplatform, Flutter.
* **Decision:** **Flutter.**
* **Rationale:** a single codebase for Android, iOS, Windows, Linux, macOS and
  Web. The ecosystem supports offline-first strategies well (such as
  Drift/SQLite), which matter for finance apps where the user may be offline at
  the moment of spending.

### 3.2. Backend: Language and Framework
* **Options:** Java (Spring Boot), Node.js, Go.
* **Decision:** **Go.**
* **Rationale:** single-binary compilation, near-zero startup time and native
  concurrency. It is an industry standard for infrastructure and modern
  fintechs, and supports a clean, high-performance architecture.

### 3.3. Database and ORM
* **Options:** MySQL, MongoDB, PostgreSQL.
* **Decision:** **PostgreSQL + GORM (MVP).**
* **Rationale:** PostgreSQL is the definitive choice for financial data
  integrity, with advanced JSONB support and rigorous relational modeling. For
  the MVP, GORM speeds up CRUD delivery.
* **Future alternative:** migrate to **SQLC** when the system needs highly
  optimized queries and strict type safety straight from SQL.

### 3.4. Authentication and Storage
* **Options:** Firebase, AWS Cognito, in-house auth (JWT), Supabase.
* **Decision:** **Supabase (Auth and Storage).**
* **Rationale:** drastically reduces the complexity of managing sessions and
  password resets in the MVP. Fully compatible with the PostgreSQL choice.
* **Future alternative:** in-house JWT if full isolation from third-party
  dependencies becomes necessary.

### 3.5. Parsing Strategy (Data Extraction)
* **Options:** direct LLM integration (OpenAI/Gemini) vs. a rule-based parser.
* **Decision:** **Regex/rule-based parser in the MVP.**
* **Rationale:** financial and engineering pragmatism. Using LLMs to categorize
  every simple expense adds unnecessary latency and per-token cost. The flow is:
  `Input -> Regex -> Rules`.
* **Future alternative:** a pluggable interface for LLMs (AI analytics) to handle
  complex inputs and descriptive financial analysis.

### 3.6. Hosting, Deployment and Observability
* **Options:** AWS (EC2/ECS), Vercel, Railway/Render.
* **Decision:** **Railway or Render (backend) + Supabase (database).**
* **Observability:** structured logs and health checks in the Go API from day
  zero.
* **Rationale:** focus on developer experience. CI/CD via GitHub Actions
  connected straight to Railway keeps the focus on code, not infrastructure,
  during the first months.

## 4. Risks and Mitigations

* **Risk:** Go's library ecosystem is smaller than Java's for some specific
  integrations.
  * **Mitigation:** isolate external integrations behind interfaces in Go, so
    in-house clients can be written cleanly when no official library exists.
* **Risk:** lock-in with Supabase Auth.
  * **Mitigation:** the backend must not couple the user ID directly to business
    rules without an abstraction layer (e.g. internal UUIDs mapped to the
    Supabase ID).
