# RFC-0001: Escolha da Stack Tecnológica Core

**Status:** Aprovado
**Data:** 03 de Julho de 2026
**Autor:** Tech Lead / Arquitetura
**Tags:** `architecture`, `stack`, `decisions`

## 1. Contexto e Problema
O projeto requer a fundação de uma plataforma financeira robusta, escalável e de longo prazo. Precisamos definir as tecnologias core que suportarão o desenvolvimento desde a fase de MVP até futuras integrações com inteligência artificial, garantindo alta performance no backend e alcance multiplataforma no frontend, sem gerar custos iniciais proibitivos.

## 2. Critérios de Decisão
* **Time-to-market do MVP:** Tecnologias que permitam iterações rápidas.
* **Escalabilidade e Performance:** Capacidade de lidar com alto volume de transações e concorrência (foco em arquitetura financeira).
* **Alcance Multiplataforma:** Menor esforço para entregar versões em diferentes sistemas operacionais.
* **Curva de Aprendizado e Valor de Portfólio:** Adoção de ferramentas altamente valorizadas pelo mercado de backend e engenharia de software moderna.
* **Pragmatismo de Custos:** Ferramentas gratuitas ou de baixíssimo custo para a fase inicial, com caminho claro para migração.

## 3. Opções Consideradas e Decisões

### 3.1. Frontend: Mobile, Desktop e Web
* **Opções:** React Native, Kotlin Multiplatform, Flutter.
* **Decisão:** **Flutter.**
* **Justificativa:** Permite uma base de código única para Android, iOS, Windows, Linux, macOS e Web. O ecossistema suporta bem estratégias offline-first (como Drift/SQLite), essenciais para aplicativos financeiros onde o usuário pode não ter conexão no momento do gasto.

### 3.2. Backend: Linguagem e Framework
* **Opções:** Java (Spring Boot), Node.js, Go.
* **Decisão:** **Go.**
* **Justificativa:** Oferece vantagens incomparáveis para este projeto: compilação em binário único, tempo de inicialização quase zero e manipulação nativa de concorrência. É o padrão da indústria para infraestrutura e fintechs modernas, garantindo uma arquitetura limpa e de altíssima performance.

### 3.3. Banco de Dados e ORM
* **Opções:** MySQL, MongoDB, PostgreSQL.
* **Decisão:** **PostgreSQL + GORM (MVP).**
* **Justificativa:** O PostgreSQL é a escolha definitiva para integridade de dados financeiros, com suporte avançado a JSONB e modelagem relacional rigorosa. Para o MVP, o GORM acelerará a entrega do CRUD.
* **Alternativa Futura:** Migração para **SQLC** quando o sistema exigir queries altamente otimizadas e type-safety rigoroso direto do SQL puro.

### 3.4. Autenticação e Storage
* **Opções:** Firebase, AWS Cognito, Autenticação Própria (JWT), Supabase.
* **Decisão:** **Supabase (Auth e Storage).**
* **Justificativa:** Reduz drasticamente a complexidade de gerenciar sessões e redefinições de senha no MVP. Totalmente compatível com a escolha do PostgreSQL.
* **Alternativa Futura:** Implementação de JWT proprietário se houver necessidade de isolamento total de dependências de terceiros.

### 3.5. Estratégia de Parsing (Extração de Dados)
* **Opções:** Integração direta com LLM (OpenAI/Gemini) vs. Parser baseado em Regras.
* **Decisão:** **Parser baseado em Regex/Regras no MVP.**
* **Justificativa:** Pragmatismo financeiro e de engenharia. Utilizar LLMs para categorizar cada despesa simples gera latência e custo por token desnecessários. O fluxo será: `Entrada -> Regex -> Regras do Banco`.
* **Alternativa Futura:** Implementação de interface plugável para LLMs (IA Analytics) para lidar com entradas complexas e análises financeiras descritivas.

### 3.6. Hospedagem, Deploy e Observabilidade
* **Opções:** AWS (EC2/ECS), Vercel, Railway/Render.
* **Decisão:** **Railway ou Render (Backend) + Supabase (Banco).**
* **Observabilidade:** Implementação desde o dia zero de logs estruturados e health checks na API Go.
* **Justificativa:** Foco em DX (Developer Experience). CI/CD automatizado via GitHub Actions conectando diretamente no Railway permite focar no código, não na infraestrutura, durante os primeiros meses.

## 4. Riscos e Mitigações
* **Risco:** Ecossistema de bibliotecas do Go ser menor que o do Java para integrações específicas.
  * **Mitigação:** Isolar integrações externas atrás de interfaces no Go, permitindo criar clientes próprios de forma limpa caso não exista uma biblioteca oficial.
* **Risco:** Lock-in com Supabase Auth.
  * **Mitigação:** O design do backend não deve acoplar o ID do usuário diretamente à regra de negócio sem uma camada de abstração (ex: usar UUIDs internos mapeados para o ID do Supabase).
  *