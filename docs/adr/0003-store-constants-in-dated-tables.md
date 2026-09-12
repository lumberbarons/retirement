---
id: ADR-0003
title: Store year-specific constants in dated tables
status: accepted
date: 2026-09-09
deciders: [lumberbarons]
supersedes: []
superseded-by: []
related: [ADR-0002]
tags: [constants, determinism, governance]
---

# ADR-0003: Store year-specific constants in dated tables

## Context and Problem Statement

The engine depends on a large set of dollar and rate figures — TFSA, RRSP,
RRIF, and CPP/OAS values; federal and Ontario brackets, credits, and surtax
thresholds; the OHP schedule; the FP Canada 2026 default assumptions — that
change every year on different index bases. If these are hard-coded inline at
each use site, the annual re-verification against CRA and the Service Canada
rate card becomes a manual search-and-replace with no way to tell which
constant applies to which year. We must choose how these values are stored
and how the engine selects the right value for an arbitrary projection year.

## Decision Drivers

- The projection spans ~50 years, so the engine must select the value in
  effect for a given future year, not a single 2026 figure.
- Correctness is gated at ±$1, and the governing spec mandates that no dollar
  constant appears outside the constants tables (except in tests).
- Every constant carries an index basis (CPI / average wage / fixed /
  plan-specific / user-set) that determines how it moves forward in time —
  see ADR-0002.
- Values must be re-verifiable against primary sources (CRA, ESDC, FP Canada)
  every January, with staleness visible, so a figure that is a year out of
  date cannot hide inside a tax function.

## Considered Options

1. **Dated tables in a central constants package** — year-keyed tables with an
   index-basis tag, a source, and a last-verified date per row
2. **Inline literals** — write each 2026 constant at its use site; update
   annually by hand
3. **Per-tax-year version packages** — a package per year of record, selected
   by a registry at run time

## Decision Outcome

Chosen option: **Dated tables in a central constants package**

The drivers that tip it: future-year selection and annual re-verification. A
year-keyed table is the only structure that tells the engine "the RRIF factor
at age 71 is fixed in law, never indexed, but the OAS clawback threshold
indexes to CPI" and lets it forward-index the latter for a year the table
doesn't yet contain. Inline literals fail both — they bury the year and the
basis, and force annual edits across the codebase. Per-tax-year version
packages solve future-year selection but at the cost of a whole package and a
registry per year for a single-project engine that neither needs historical
reproduction across many law regimes nor has external consumers pinning
versions; the dated table gives the same selection with one lookup.

## Consequences

### Positive

- Single source of truth for every figure; a yearly update is a table edit
  plus a re-verification of the source/date fields, not a code-wide sweep.
- A constant's year, basis, source, and last-verified date are visible where
  it is declared, so staleness and mis-tagging are reviewable in a diff.
- The ±$1 gate and the "no constants outside the tables" rule become
  mechanically checkable (a lint or a test) rather than a convention.

### Negative

- More boilerplate: a table type, the base 2026 rows, and a lookup path must
  exist before any tax code runs.
- The forward-indexing rule (how a CPI-based constant advances past the newest
  table row) must be chosen and tested — the engine cannot just error on a
  missing future year.

### Risks

- A wrong forward-indexation rule would drift every CPI- or wage-based figure
  silently for decades and still pass "looks right" review; the mitigation is
  a dedicated lookup test against the index-basis registry, not just spot
  checks.

## Pros and Cons of the Options

### Dated tables in a central constants package

- Good: year/basis/source/verified-date per row; one lookup; diffable against
  primary sources; enables a staleness lint.
- Bad: boilerplate before first use; the forward-indexation rule is a new
  thing to get right.

### Inline literals

- Good: zero setup; the fastest way to get a first projection running.
- Bad: year and basis buried at the use site; annual update is error-prone
  search-and-replace; staleness is invisible; fails the spec's "no constants
  outside the tables" rule.

### Per-tax-year version packages

- Good: crisp year selection; clean snapshot of each tax year.
- Bad: a package and registry per year is heavy for one engine with no
  external pinning need; duplicated structure that the dated table collapses
  into one lookup.

## Implementation Notes

- Single package owns all constants; every row carries `yearEffective`,
  `value`, `indexBasis` (CPI | averageWage | fixed | planSpecific | userSet),
  `source` (e.g. CRA, ESDC, FP Canada), and `lastVerified`.
- Selection = exact row for the projection year if present, else forward-index
  from the newest row by the row's `indexBasis`; a `fixed`-basis row is
  returned unchanged for any later year.
- User overrides (FP Canada returns/inflation, spending inflation) layer over
  the base table and never alter the base rows themselves.
- Invariants future changes must respect: no dollar or rate constant is
  declared outside this package; the forward-indexation logic is the only
  place a raw table value is extrapolated; tests may copy a canonical value
  but not re-derive it.
- Add a staleness lint (in CI) that flags any row whose `lastVerified` is more
  than 12 months old, matching the spec's annual re-verification cadence.

## References

- Ontario Retirement Simulator Spec §6.2 (index-basis registry), §7 (default
  FP Canada 2026 assumptions, user-overridable), §9 (indexation governance,
  annual re-verification)
- Deterministic Retirement Engine spec — task T004 and the Done-When item
  requiring no dollar constant outside the dated tables
- ADR-0002 (nominal engine; index basis per constant)