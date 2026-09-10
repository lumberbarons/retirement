# AGENTS.md

## Specs

| Spec | What it governs |
|---|---|
| `ontario-retirement-simulator-spec.md` | The governing implementation spec: the annual order of operations, every 2026 constant and its index basis, and the §7 validation cases (must pass within ±$1). Source of truth for tax/benefit arithmetic. |
| `specs/001-deterministic-retirement-engine/spec.md` | The accepted epic for the deterministic engine — scope, user stories, task list, and done-when criteria. |

Glossary of acronyms: `docs/glossary.md`.

## Architectural Decisions

ADRs live in `docs/adr/`. Read the relevant ADRs before proposing architectural changes — they encode constraints and rejected alternatives. When writing or modifying a spec, cite the ADRs that constrained it in the spec's own frontmatter; ADRs do not track their downstream consumers.

| ADR | When this applies |
|---|---|
| `docs/adr/0001-use-go-for-engine-and-cli.md` | Any change to the engine's language/runtime, the `retire` CLI packaging/distribution, or the `internal/` package layout (engine-vs-CLI separation). |
| `docs/adr/0002-use-nominal-dollar-engine.md` | Any change to how dollar amounts, inflation, or indexation are handled — the nominal-vs-real frame, constant index bases, or where deflation happens. |
| `docs/adr/0003-store-constants-in-dated-tables.md` | Any addition, removal, or re-verification of tax/benefit/assumption figures, or changes to how constants are selected for a projection year. |
| `docs/adr/0004-use-float64-for-money.md` | Any change to how dollar amounts are represented, rounded, or compared — money types, cent-rounding boundaries, or equality checks. |