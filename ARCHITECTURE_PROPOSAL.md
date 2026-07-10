# Dewey — Architecture Proposal

_Status: proposal / for discussion — 2026-07-10_

This document reviews Dewey's current architecture against current (2025–2026)
Go design guidance and proposes a **proportionate** evolution. Dewey is a small,
single-operator school-library app, not a microservice fleet — so the golden
rule below shapes every recommendation.

> **Simplicity first, complexity only when it earns its place.** The most common
> failure mode in Go projects is over-engineering the structure up front: deep
> package trees, premature interfaces, DDD/hexagonal ceremony on a CRUD app.
> Start with what today needs and refactor as pressure appears.
> — synthesised from the sources listed at the end.

Because of that rule, this proposal **explicitly rejects** a full Clean/Hexagonal
rewrite, a DI framework (wire/fx), CQRS, or a `pkg/` public API. None are
justified at this size. What follows are targeted, high-leverage changes.

---

## 1. What Dewey is today

- **Shape:** a server-rendered monolith. Go + `html/template`, `chi` router,
  SQLite (`modernc.org/sqlite`, pure-Go, no cgo), an ESP8266 NFC reader over a
  serial port, and the Google Books API for ISBN metadata. German UI.
- **Scale:** ~2,500 lines of Go across `internal/{db,store,web,nfc,openlibrary,seed}`.
- **Deployment:** one process, one operator (a librarian/teacher), a **live**
  `dewey.db` holding real data. WAL mode, `foreign_keys` enforced per-connection.
- **Tests:** Playwright e2e only (`e2e/`) against a seeded throwaway server.
  **There are no Go unit tests.**

### Current layering (already sound)

```
main.go                 composition root: flags/env → db → store → nfc → web server
  └─ internal/db        schema (const SQL) + Open() + ad-hoc migration func
  └─ internal/store     data access: Store{ *sql.DB }, one file per entity, raw SQL
  └─ internal/web       chi router + http handlers + html/template rendering
  └─ internal/nfc       serial Reader with pub/sub (Subscribe/SubscribeStatus)
  └─ internal/openlibrary  external ISBN lookup (NB: actually calls Google Books)
  └─ internal/seed      mock data for dev/e2e
```

This is a healthy foundation: `internal/` enforces encapsulation, dependencies
flow one direction (`web → store → db`), constructor injection (`store.New`,
`nfc.New`, `web.NewServer`) is already the norm, and every DB call takes a
`context.Context`. Most of what follows is refinement, not restructuring.

---

## 2. Assessment against current Go guidance

| Area | Today | Guidance (2025–2026) | Verdict |
|------|-------|----------------------|---------|
| Package layout | `internal/` split by concern | Keep it simple; `internal/` for private code | ✅ Good |
| Dependency injection | Manual constructor injection | Manual constructor DI is the idiomatic default | ✅ Good |
| Context | **Stored in `Server` struct** | Never store `ctx` in a struct; pass it per-call | ⚠️ Fix |
| Data access | Concrete `*store.Store`, no interfaces | Define narrow interfaces **at the consumer** for testability | ⚠️ Improve |
| Business logic | Split between handlers and store | Keep a clear seam between transport, domain, and data | ⚠️ Improve |
| Migrations | `const schema` + hand-written `addCascadeToBookPositions` | Versioned, ordered migrations — critical once the DB is live | ⚠️ Fix |
| Logging | stdlib `log` | `log/slog` (structured, stdlib, zero new deps) | ⚠️ Improve |
| Server lifecycle | `http.ListenAndServe` directly | `http.Server` with timeouts + graceful shutdown | ⚠️ Fix |
| Error → HTTP | `http.Error(w, err.Error(), 500)` repeated | Centralise; don't leak raw internal errors to users | ⚠️ Improve |
| Adapter naming | package `openlibrary` calls **Google Books** | Names must not lie; treat as a swappable port | ⚠️ Fix |
| Testing | e2e only, no unit tests | Interfaces + a service layer make unit tests trivial | ⚠️ Improve |

---

## 3. Target architecture

A light **three-seam** model — transport / domain / data — that keeps Go's
"accept interfaces, return structs" idiom and adds no new frameworks.

```
                 ┌──────────────────────────────────────────────┐
   HTTP / SSE ───▶  transport (internal/web)                     │
                 │  - routing, form parsing, template rendering  │
                 │  - maps domain errors → HTTP status + message │
                 └───────────────┬──────────────────────────────┘
                                 │ calls interfaces it declares
                 ┌───────────────▼──────────────────────────────┐
                 │  domain / service (internal/library)          │
                 │  - checkout/return/prolong rules, "delete book│
                 │    only if no copies remain", scan resolution │
                 │  - depends on Repository + BookLookup ports    │
                 └───────────────┬──────────────────────────────┘
                                 │ interfaces
        ┌────────────────────────┼───────────────────────────────┐
        ▼                        ▼                                ▼
  data (internal/store)   book metadata (internal/booksapi)   hardware (internal/nfc)
  SQLite repository        Google Books adapter                serial reader (unchanged)
```

Key idea: **handlers orchestrate HTTP, the service owns the rules, the store owns
SQL.** Today the middle seam is implicit — some rules live in handlers
(`handleCopyDelete` decides to delete the book when the last copy goes;
`handleBookDetail` computes overdue/reserved view state) and some live in the
store (`ProlongLoan`, `DeleteBook`, `ScanByNFC`). Making the seam explicit is the
single biggest structural win and the thing that unlocks unit testing.

For a project this size the service layer can be **one package** (`internal/library`),
not one-package-per-aggregate. Grow it only if it gets unwieldy.

---

## 4. Recommendations (prioritised)

### P1 — Correctness & operational safety

1. **Versioned migrations.** The DB is live, so `CREATE TABLE IF NOT EXISTS` +
   one bespoke rebuild function won't scale and risks divergence between fresh
   and existing databases. Adopt an ordered, tracked scheme:
   - a `schema_migrations(version)` table, `//go:embed migrations/*.sql` files
     applied in order inside a transaction, **or**
   - a small library (`pressly/goose` or `golang-migrate`) if you'd rather not
     hand-roll it.
   Fold the existing schema as migration `0001` and the CASCADE rebuild as
   `0002`. This directly serves the "changes need a migration path" constraint.

2. **Graceful shutdown + server timeouts.** Replace `http.ListenAndServe` with an
   `http.Server{ReadHeaderTimeout, ReadTimeout, WriteTimeout, IdleTimeout}` and
   shut it down on `SIGINT`/`SIGTERM` via `signal.NotifyContext`, then
   `srv.Shutdown(ctx)`, then `nfcReader.Stop()` and `db.Close()`. On a WAL SQLite
   DB with a background serial goroutine, clean shutdown avoids truncated
   writes and leaked ports. (Note: SSE handlers hold connections open — give
   `WriteTimeout` a generous value or scope timeouts per-route.)

3. **Don't store `context.Context` in `Server`.** `server.go` keeps a long-lived
   `ctx` field for the NFC goroutine. Pass that context explicitly to whatever
   starts background work (it already flows into `nfc.Start`), and use
   `r.Context()` in handlers (which it already does). Removing the field
   eliminates a documented anti-pattern and a subtle lifetime foot-gun.

### P2 — Structure & testability

4. **Introduce `internal/library` (service layer).** Move rules out of handlers:
   the "last copy deleted ⇒ delete book" flow, scan-result assembly, and the
   overdue/reserved computations in `handleBookDetail`. Handlers shrink to
   parse → call service → render. The store stays a thin SQL repository.

5. **Declare interfaces at the consumer.** In `internal/library`, define the
   narrow ports it actually needs, e.g.:
   ```go
   type Repository interface {
       GetBook(ctx context.Context, id int64) (store.Book, error)
       ListCopiesByBook(ctx context.Context, id int64) ([]store.Copy, error)
       DeleteBook(ctx context.Context, id int64) error
       // ...only what the service uses
   }
   type BookLookup interface {
       Lookup(ctx context.Context, isbn, apiKey string) (BookInfo, error)
   }
   ```
   `*store.Store` satisfies `Repository` for free ("accept interfaces, return
   structs"). This makes the service unit-testable with a fake repo — no SQLite
   needed — which is the payoff the e2e suite can't give you (fast, focused,
   rule-level tests).

6. **First Go unit tests.** Once (4)+(5) land, cover the rule-heavy paths:
   `ProlongLoan` reservation blocking, `DeleteBook` on-loan rejection, scan
   resolution, and checkout due-date math. These are exactly the bugs e2e is
   slow at catching.

### P3 — Idiomatic polish (low effort, clear payoff)

7. **Switch to `log/slog`.** Structured, leveled, in the stdlib since 1.21 —
   zero new dependencies. Replace `log.Printf("info: …")` /
   `log.Fatalf` with a `*slog.Logger` injected from `main`. Pass it into
   `web`/`nfc`/`library` instead of using the package-global logger.

8. **Rename the metadata adapter.** Package `openlibrary` calls the **Google
   Books** API — the name misleads every reader. Rename to `internal/booksapi`
   (or `bookmeta`) and treat it as a swappable port behind the `BookLookup`
   interface. If Open Library is ever wanted as a fallback, it slots in cleanly.

9. **Centralise error → HTTP mapping.** The `http.Error(w, err.Error(), 500)`
   pattern is repeated dozens of times and leaks raw internal/SQL error text to
   end users (a librarian shouldn't see driver strings). Add one helper that
   maps sentinel domain errors (`ErrNotFound`, `ErrBookOnLoan`, `ErrLoanReserved`,
   `ErrInvalidDueDate`) to status codes and logs the detail server-side while
   showing a friendly German message.

10. **A `Config` struct + `run() error` in main.** Collect the flags/env
    (`addr`, `dbPath`, `seed`, NFC port) into one `Config`, and have `main` call
    `if err := run(cfg); err != nil { … }`. Makes `main` linear and, combined
    with the above, makes the whole startup path testable.

### P4 — Optional / defer

- **`cmd/dewey/main.go`.** The standard layout puts entry points under `cmd/`.
  Harmless and conventional, but pure churn for a single-binary app — do it only
  if a second binary (e.g. a CLI/import tool) ever appears.
- **Store boilerplate.** The repeated `COALESCE(...,'')` projections and
  RFC3339 text↔`time.Time` conversions in `store/loans.go` are candidates for
  small shared helpers, but they're readable as-is. Low priority.
- **Repository interface for the whole store.** Don't define one giant
  `StoreInterface`. Keep interfaces small and consumer-defined (see #5).

---

## 5. Suggested sequencing

Each phase is independently shippable and leaves the app releasable.

1. **Phase 1 (safety):** versioned migrations, graceful shutdown, drop the stored
   `ctx`. _Highest risk-reduction, touches `db` and `main` only._
2. **Phase 2 (polish):** `slog`, rename `openlibrary → booksapi`, centralised
   error mapping, `Config`/`run()`. _Mechanical, low-risk, improves every file
   it touches._
3. **Phase 3 (structure):** extract `internal/library`, consumer-side interfaces,
   move rules out of handlers, add the first unit tests. _Biggest change; do it
   last, on top of a clean base._

---

## 6. Explicit non-goals

- No Clean/Hexagonal/DDD framework, no CQRS, no event sourcing.
- No DI framework (wire/fx) — manual constructor injection stays.
- No microservices, no message bus, no `pkg/` public API.
- No swap away from SQLite or `html/template`; both fit the deployment well.

These are the right call at Dewey's size; revisit only if the app grows a second
deployment target, multiple concurrent operators, or a public API.

---

## Sources (current Go guidance)

- [Go Project Structure 2026: Clean Architecture and Best Practices — Reintech](https://reintech.io/blog/go-project-structure-2026-clean-architecture-best-practices)
- [Introducing Clean Architecture in Go — Three Dots Labs](https://threedots.tech/post/introducing-clean-architecture/)
- [Go Project Structure: Practices & Patterns — Rost Glukhov](https://medium.com/@rosgluk/go-project-structure-practices-patterns-7bd5accdfd93)
- [Repository pattern in a Go service — Paweł Grzybek](https://pawelgrzybek.com/repository-pattern-in-go-service/)
- [How to Implement the Repository Pattern in Go — OneUptime (Jan 2026)](https://oneuptime.com/blog/post/2026-01-07-go-repository-pattern/view)
- [Dependency Injection in Go: Patterns & Best Practices — Rost Glukhov (Dec 2025)](https://www.glukhov.org/post/2025/12/dependency-injection-in-go/)
- [Idiomatic Go Programming Guide 2025+ (Gold Standard)](https://gist.github.com/ashokallu/26445f706d33308b0f243f76618cb687)
- [Design Patterns in Go — Refactoring.Guru](https://refactoring.guru/design-patterns/go)
</content>
</invoke>
