---
id: ADR-0001
title: Use Go for the engine and CLI
status: accepted
date: 2026-09-09
deciders: [lumberbarons]
supersedes: []
superseded-by: []
related: [ADR-0004]
tags: [language, stack, distribution]
---

# ADR-0001: Use Go for the engine and CLI

## Context and Problem Statement

The deterministic retirement engine is greenfield: the repo currently holds
only the governing implementation spec and the accepted engine spec, no code.
Before any scaffolding we must pick the language and runtime for the engine
and its CLI. The decision shapes every later choice — the annual projection
loop (run to second death, ~50 years), the gross-vs-net bisection, the ±$1
validation gate, and eventually Phase-4 Monte Carlo — so it should be made
and recorded once rather than assumed.

## Decision Drivers

- The engine runs a full projection once per year of the horizon, and Phase 4
  (Monte Carlo / historical bootstrap) will run thousands to hundreds of
  thousands of full projections to report success probability. One projection
  must stay cheap enough that Monte Carlo remains interactive; a compiled
  language keeps this off the critical path, an interpreted one strains it.
- The primary deliverable is a distributable CLI (`retire project --config
  household.example.yaml`) plus a CSV report. A single static binary is the
  lowest-friction way to hand that to a non-developer household planner.
- Correctness is defined as every §7 validation case passing within ±$1 in
  CI. The language should make that cheap and idiomatic — table-driven tests,
  zero build config, fast test runs.
- The engine must be reusable behind an HTTP layer later without a rewrite,
  so it must be structured library-first with the CLI as a thin shell.
- A standing project convention: for a new backend with no obvious best
  practice, prefer Go.

## Considered Options

1. **Go** — engine as an `internal/...` package tree plus a thin `cmd/retire` CLI
2. **Python (uv)** — prototype the engine as a Python module with a `retire` console script
3. **TypeScript (Node)** — engine in TS so the future web frontend shares one language

## Decision Outcome

Chosen option: **Go**

Go is the only option that satisfies the distribution and Monte-Carlo
performance drivers while matching the standing backend convention. Python
would be the fastest to prototype and has the strongest numeric ergonomics
(`decimal`), but a planning tool handed off as a single binary argues against
requiring a uv/Python runtime, and running hundreds of thousands of
projections in pure Python becomes a real perf concern. TypeScript would
unify language with the future React frontend, but JS has no native decimal
type — like Go's own eventual float64 choice, it would still need a
follow-on money-representation decision, without Go's static typing or
table-driven test ergonomics to lean on while making it. Go's `go test`
table-driven tests also make the validation gate near-zero-friction compared
to wiring a test runner in the other two.

## Consequences

### Positive

- Single static binary ships `retire` to any macOS/Linux user with no runtime.
- `go test` gives the ±$1 validation suite for free, fast enough to run every
  case on every commit.
- Strong static typing forces the config, money, and tax structs to be
  explicit, which matters in a codebase dominated by fiddly 2026 constants.
- Library-first layout keeps the engine reusable behind a future web frontend.

### Negative

- No built-in arbitrary-precision decimal: money arithmetic will be float64 or
  integer cents. That is a deliberately *un*resolved follow-on decision, not
  something this ADR silently settles.
- Go's verbosity and explicit error handling cost some development speed in a
  large tax engine versus Python.

### Risks

- Float64 cross-platform determinism is not guaranteed. The money/rounding
  decision this ADR defers must address it before the validation gate is
  trustworthy on multiple machines; until then CI pins a platform.
- If the future web frontend later needs browser-side validation, this ADR's
  server-only engine cannot be reused client-side without WASM or a rewrite.
  Current intent (report is CLI + CSV) makes that unlikely.

## Pros and Cons of the Options

### Go

- Good: compiled single binary; idiomatic table-driven tests for the ±$1 gate;
  fast enough for Phase-4 Monte Carlo; matches standing backend convention.
- Bad: no stdlib decimal; more verbose than Python for a tax engine.

### Python (uv)

- Good: fastest to draft; `decimal.Decimal` for exact money; richest numeric
  ecosystem.
- Bad: distributing the CLI requires a uv/Python runtime; pure-Python Monte
  Carlo at scale is slow; no static types without extra scaffolding.

### TypeScript (Node)

- Good: shares a language with the future React frontend; large ecosystem.
- Bad: no native decimal type, so it inherits the same deferred
  money-representation question as Go without Go's static typing or
  table-driven tests to offset it; CLI distribution requires a Node runtime
  or heavy packaging.

## Implementation Notes

- Structure the module library-first: domain packages (`config`, `constants`,
  `projection`, `tax`, `benefits`, `accounts`, `pension`, `validate`,
  `report`) under an `internal/` tree, with `cmd/retire/main.go` the only
  place that does terminal/IO. This is what lets a later web server import the
  engine without pulling in CLI concerns.
- Invariants future changes must respect: engine packages never write to
  stdout/files directly (output goes through the report and config packages),
  and no package-level mutable state, so parallel Monte Carlo runs cannot race.
- Numeric representation of money is a separate, follow-on ADR. Every package
  must treat money types as a single shared abstraction from day one so that
  decision lands in one place rather than per-package.
- Revisit this decision if Monte Carlo throughput becomes a bottleneck despite
  Go, or if browser-side execution of the engine becomes a real requirement.

## References

- Ontario Retirement Simulator Spec — governing implementation spec (build
  plan phases, §7 validation cases)
- Deterministic Retirement Engine spec — the epic this ADR governs (stack
  assumption noted in its Assumptions)
- Repo developer guidance (CLAUDE.md) — "for a new backend with no obvious
  best practice, prefer Go"