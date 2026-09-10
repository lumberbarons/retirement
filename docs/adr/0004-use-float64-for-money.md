---
id: ADR-0004
title: Use float64 for money, rounded to cents at boundaries
status: proposed
date: 2026-09-09
deciders: [lumberbarons]
supersedes: []
superseded-by: []
related: [ADR-0001, ADR-0002]
tags: [money, determinism, performance]
---

# ADR-0004: Use float64 for money, rounded to cents at boundaries

## Context and Problem Statement

ADR-0001 deferred the question of how dollar amounts are represented in the
engine. The projection loops a stated number of dollars through returns,
taxes (brackets, surtax, health premium, dividends, capital gains), benefits,
and account mechanics over ~50 years, and the whole thing is validated only
to ±$1. We must pick a numeric representation that is fast enough for Phase-4
Monte Carlo, keeps the tax math readable, and cannot drift visibly past the
±$1 bar over five decades of compounding.

## Decision Drivers

- The only correctness bar is ±$1 per validation case — not cent-exactness.
  No money amount must ever be reproduced to the penny by an external auditor.
- The annual loop is percentage-heavy: bracket rates, dividend gross-ups
  (38% / 15%) and DTCs (15.0198%, 10.0%), surtax (20% / 36%), OHP ramps,
  RRIF factors (5.28%, …), inflation and return assumptions. These are
  naturally continuous, not integer, quantities.
- Phase 4 (Monte Carlo / historical bootstrap) runs thousands to hundreds of
  thousands of full projections; a representation that is ~100x slower than
  native arithmetic becomes the bottleneck there.
- The engine must be deterministic enough that a validation case gives the
  same result on a re-run and on another machine running the same Go version.

## Considered Options

1. **float64 + cent-rounding at boundaries** — compute in float64, round to
   the nearest cent only at defined boundaries (roll-forward, reported output)
2. **Integer cents (int64) fixed-point** — all money as integer cents, a fixed
   scale for rate math, an explicit rounding policy
3. **Decimal library** (`math/big.Rat` or Shopspring-style decimal) — exact
   base-10 arithmetic

## Decision Outcome

Chosen option: **float64 + cent-rounding at boundaries**

The ±$1 driver is decisive: at that bar, float64's sub-cent representation
error is far inside tolerance, so the exactness integer or decimal arithmetic
buys is not needed. The percentage-heavy, continuous nature of tax math makes
float64 the natural and most readable fit, and it is the only option whose
performance keeps Monte Carlo off the critical path. Integer cents is exact
but forces a fixed scale and rounding policy onto gross-ups and DTCs that a
tax engine should be able to express as plain decimals; decimal libraries are
exact but ~100x slower and add an external dependency for a precision level
the validation gate does not ask for.

## Consequences

### Positive

- Tax math stays direct and auditable against the spec's own decimal figures.
- Native float64 speed makes Phase-4 Monte Carlo tractable with no
  representation-induced overhead.
- One shared rounding helper gives a single place where "a money value becomes
  exact to the cent" is defined.

### Negative

- We gave up cent-exactness and cross-platform *bit-identical* results; results
  are equal within well under a cent, but engineers must not assert dollar
  equality with `==`.
- Every engineer must know *where* rounding is allowed, or cents drift
  haphazardly at different points in the loop.

### Risks

- Floating-point drift across 50 years is the thing to watch; the mitigation
  is rounding to the cent at the account roll-forward and reported-output
  boundaries each year, plus a validation case around the full-horizon total,
  which bounds cumulative drift far below $1.
- This closes the cross-platform risk ADR-0001 flagged and deferred to this
  decision: float64 results are not guaranteed bit-identical across platforms,
  and this ADR does not make them so. Instead, the ±$1 gate makes that
  irrelevant — cross-platform drift stays far below a cent, well inside
  tolerance, once the rounding boundaries above are respected. CI's
  platform pin (added as a stopgap under ADR-0001) is no longer required for
  validation-gate trustworthiness and may be dropped.

## Pros and Cons of the Options

### float64 + cent-rounding at boundaries

- Good: readable tax math; native speed for Monte Carlo; error well inside
  the ±$1 bar once boundaries are respected.
- Bad: no cent-exactness or guaranteed bit-identical cross-platform results;
  requires discipline about `==` and about rounding points.

### Integer cents (int64) fixed-point

- Good: exact and deterministic; arguably the safest single-choice default.
- Bad: gross-ups/DTCs/ramps need a fixed scale and a rounding policy; more
  verbose and error-prone for a percentage-heavy engine; integer overflow is
  only averted by `int64` with a cents scale (~$9.2e16 range is ample, but a
  scale still has to be maintained everywhere).

### Decimal library

- Good: exact base-10; matches how money is usually reasoned about.
- Bad: ~100x slower — a real cost for Monte Carlo; external dependency or the
  heavyweight `math/big.Rat`; unnecessary precision for a ±$1 gate.

## Implementation Notes

- Introduce a single shared rounding helper (round-to-nearest-cent, half-up)
  and one place where a computed money value is committed to cents: the annual
  roll-forward and the reported output. No other code rounds money.
- Never compare money with `==`; compare after cent-rounding, or within an
  epsilon chosen well below the $1 tolerance (e.g. $0.005).
- The gross-vs-net bisection converges to $1 on the *net spending* figure, so
  its stop condition is unaffected by sub-cent representation error.
- Revisit this decision to cent-exact integer or decimal arithmetic only if a
  future requirement (audited reconciliation, tax-filing output) demands
  penny-exactness that the ±$1 gate explicitly does not.

## References

- Ontario Retirement Simulator Spec §7 (validation cases land within ±$1)
- ADR-0001 (engine/CLI — deferred this decision), ADR-0002 (nominal engine)