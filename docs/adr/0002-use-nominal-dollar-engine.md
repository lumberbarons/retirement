---
id: ADR-0002
title: Compute in nominal dollars and deflate at the reporting layer
status: accepted
date: 2026-09-09
deciders: [lumberbarons]
supersedes: []
superseded-by: []
related: [ADR-0003, ADR-0004]
tags: [architecture, money, determinism, constants]
---

# ADR-0002: Compute in nominal dollars and deflate at the reporting layer

## Context and Problem Statement

The engine projects a household's finances over a ~50-year horizon, and
Canadian retirement arithmetic is full of constants that move on *different*
bases: tax brackets and credits index to CPI, YMPE and the RRSP dollar limit
index to average wage growth, and a long list of figures never index at all
(non-indexed DB pensions, the frozen Ontario $150k/$220k brackets, the
never-indexed $2,000 federal pension amount, the never-indexed $20,000 OHP
floor). We must choose whether the engine computes in real (today's) dollars
or nominal (current-year) dollars, because getting this wrong is a source of
silent, compounding correctness bugs.

## Decision Drivers

- Published validation cases are all in nominal dollars: the RRIF minimum is
  `Jan-1 balance × factor` on a nominal balance, the OAS clawback ceilings
  are nominal ($154,708 / $160,647), CPP caps are nominal. A real-dollar
  engine would have to re-derive every one of these through a deflator before
  it could even be compared to the spec.
- Correctness is gated at ±$1 against those nominal cases. Any per-item
  deflation done inside the loop is a chance to mismatch the spec by more than
  a dollar and fail the gate.
- The report must present every year in *both* nominal and today's dollars.
  Two views are required regardless of internal representation, so the
  internal choice is about correctness, not about what the user sees.
- The index basis of each constant is already fixed by the governing spec as a
  named registry (CPI / average wage / fixed / plan-specific / user-set); the
  engine has to track that basis no matter which unit it computes in.

## Considered Options

1. **Nominal engine** — compute in current-year dollars; every constant carries
   an index basis; deflate once at the reporting layer
2. **Real-dollar engine** — deflate all inputs to today's dollars, compute in
   real terms, re-inflate for report output
3. **Hybrid** — some quantities tracked in real, some in nominal, converted ad hoc

## Decision Outcome

Chosen option: **Nominal engine**

The nominal cases in the validation gate are the deciding driver: they are the
project's definition of correct, and they are stated in nominal dollars, so
computing in nominal dollars makes "is this right?" a direct comparison rather
than a round-trip through a deflator. The real-dollar engine looks simpler but
forces a custom deflator for every non-CPI-indexed item — that is precisely
where the bugs breed, and it buys nothing here because two report views are
required anyway. The hybrid option is rejected outright: the index-basis
registry already imposes uniform bookkeeping, and a mixed engine reintroduces
the per-item-deflator bug surface while keeping the reporting-layer conversion.

## Consequences

### Positive

- Every validation case maps directly to a nominal computation; the ±$1 gate
  compares like for like.
- One index basis per constant, one conversion for reporting — bugs localize to
  the single point where a constant is mis-tagged or the report deflates.
- Deflation is a display concern and can be added, changed, or dropped without
  touching the tax/benefit arithmetic.

### Negative

- User inputs that are naturally stated in today's dollars (the spending
  target) must be converted to nominal before the loop runs.
- There is no "one truth" number in the loop itself: nominal values change
  every year, so anyone reading intermediate state must remember which dollars
  they are looking at.

### Risks

- "Today's dollars" columns could be wrong if the report-layer deflation uses
  a different factor than the constants used. The underlying nominal result is
  still correct; the mitigations are a single deflation primitive and a test
  that the report's real column equals nominal ÷ CPI for a known year.

## Pros and Cons of the Options

### Nominal engine

- Good: matches the validation gate directly; minimal moving parts; report
  layer trivially produces both views.
- Bad: spending and other "today's-dollar" inputs need an explicit real→nominal
  step; intermediate nominal state is easy to misread.

### Real-dollar engine

- Good: intermediate state is in a "real" frame that reads naturally to a human.
- Bad: needs a deflator per non-CPI-indexed constant; validation cases require
  round-tripping; diverges from how every published figure is stated.

### Hybrid

- Good: none beyond marginal convenience in isolated modules.
- Bad: worst of both — carries the per-item deflator surface and the report
  conversion, with no single authority on which frame a value sits in.

## Implementation Notes

- Every dollar constant lives in a dated table keyed by year and carries an
  index-basis tag (CPI, average wage, fixed, plan-specific, user-set). This is
  the enforcement mechanism for "nominal, with a known basis".
- Deflation happens in exactly one place — the report/display layer — using a
  single CPI factor series. No other package may convert between nominal and
  real.
- The real→nominal conversion for the spending target is a distinct, separate
  concern: spending inflation is independent of tax-bracket indexation, so it
  must not reuse the report-layer deflator.
- Revisit this decision if the engine ever needs to compare purchasing power
  across eras *inside* the loop (e.g. terminal-status scoring mid-projection),
  which a nominal engine does not naturally support.

## References

- Ontario Retirement Simulator Spec §6.1 (real→nominal conversion), §6.2
  (nominal engine, deflate at reporting layer, index-basis registry), §6.5
  (spending inflation ≠ bracket indexation), §7 (nominal validation cases)
- Deterministic Retirement Engine spec — Prior Decisions (nominal dollars,
  deflate once at reporting layer)