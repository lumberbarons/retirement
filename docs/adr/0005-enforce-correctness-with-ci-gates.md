---
id: ADR-0005
title: Enforce numerical correctness with executable CI gates
status: accepted
date: 2026-09-12
deciders: [lumberbarons]
supersedes: []
superseded-by: []
related: [ADR-0001, ADR-0002, ADR-0003, ADR-0004]
tags: [testing, determinism, governance]
---

# ADR-0005: Enforce numerical correctness with executable CI gates

## Context and Problem Statement

The engine's deliverable is a ~50-year financial plan, and its characteristic
failure is not a crash but a plausible wrong number: a mis-indexed bracket, a
flipped survivor factor, or a broken solver premise produces output that reads
correctly and is wrong by thousands of dollars. There is no auditor or
reconciliation downstream to catch it; the only external truth anchors are the
governing spec's §7 validation cases, currently ported in `internal/validate`.
Verification today is hand-written unit tests plus a 90% coverage gate and code
review — and coverage proves a line ran, not that a test would notice the
number changing. With a queue of arithmetic features ahead (government
benefits, DB pensions, account mechanics, survivorship, taxable yields, the
CLI), the project needs a correctness policy that CI enforces and that grows
with the engine, rather than one that depends on a reviewer or agent
remembering to extend it.

## Decision Drivers

- The failure mode is silent: a wrong projection looks like a right one, and
  nothing downstream reconciles it. A crash is cheap; a plausible wrong total
  is the expensive defect.
- The ±$1 §7 gate is the project's only definition of correct (ADR-0002), and
  it is a sample of values, not a proof of behavior across the input space.
- The engine is deterministic by construction (ADR-0002 nominal dollars,
  ADR-0004 cent-rounding at roll-forward), so identical input yields identical
  output — determinism, conservation, and golden checks are meaningful and
  stable.
- Queued features (#16 benefits, #17 DB pensions, #18 account mechanics,
  #19 survivorship, #21 CLI, #27 taxable yields) each add arithmetic branches
  and config surface. Enforcement must scale with them at zero per-feature
  process cost.
- Humans and agents work this repo asynchronously; a rule that depends on
  someone remembering it during review is not a gate.
- CI already runs test, vet, gofmt, a >90% coverage gate, and govulncheck on
  every PR, so stronger gates are incremental, not new infrastructure.
- Phase 4 Monte Carlo (ADR-0001) runs thousands of full projections; whatever
  runs per-PR must stay cheap, with the expensive verification done on a
  schedule.
- Expected values can only come from outside the engine (the governing spec,
  CRA/ESDC examples, a second implementation). The policy must keep external
  truth distinct from self-consistency so the suite cannot certify its own
  bugs.

## Considered Options

1. **Layered executable gates** — a scenario registry applying oracle,
   invariant, and golden checks; a mutation gate on arithmetic packages;
   boundary fuzzing; a nightly soak that files issues
2. **Status quo** — hand-written unit tests plus the coverage gate and review
3. **Oracle-only** — expand §7-style cases and require one per feature
4. **Invariant-only** — property-based tests over generated configs
5. **Golden-only** — freeze outputs of representative scenarios
6. **Formal methods** — machine-checked proofs of the tax functions

## Decision Outcome

Chosen option: **Layered executable gates**

Only the layered strategy satisfies all four needs at once: pin true values
against an external oracle, check the system invariants the oracle cases do
not touch, detect value drift when behavior changes unintentionally, and force
the suite itself to grow. Each single-technique option fails at least one:
status quo relies on vigilance and a coverage number that cannot detect a
wrong assertion; oracle-only cannot be extended without a human deriving new
expected values and never checks invariants such as the solver's monotonicity
premise; invariant-only can pass while every tax formula is wrong; golden-only
freezes current behavior, bugs included. Formal methods would give the
strongest guarantee but are disproportionate for a ±$1 personal planning
tool. The mutation gate is what specifically answers "no one has to remember":
a new arithmetic branch that no test can distinguish fails CI by construction.

## Consequences

### Positive

- Every registered scenario inherits oracle, invariant, and golden checks;
  adding coverage becomes data entry, not test-writing.
- New arithmetic cannot merge without a test that discriminates its output,
  and new config surface cannot merge without a scenario (field coverage).
- An arithmetic change that moves a number shows up twice: as a golden diff
  and as a required spec or validation-case change.
- The bisection's monotonicity premise (`internal/projection/solve.go`) is
  continuously tested rather than assumed; if a future benefit interaction
  breaks it, CI fails instead of silently returning a wrong withdrawal.
- The nightly soak surfaces deep-horizon and crash defects without anyone
  watching, and files them into the existing work queue.
- Coverage stays as a floor and is no longer asked to carry correctness.

### Negative

- CI becomes slower and more complex; mutation testing in particular is a new
  tool to configure and maintain.
- Golden updates add friction to deliberate arithmetic changes, and a
  one-keystroke `-update` can bless a bug if the diff is not read; the
  spec/validation-change requirement is what keeps this honest.
- Some redundancy: a value may be pinned by an oracle case, an invariant, and
  a golden, so one defect can fail several gates.
- Property and field-coverage tests grow with the config schema and must be
  kept in step with it.

### Risks

- Equivalent mutants and slow runs could tempt the team to widen mutation
  exclusions until the gate is meaningless. Mitigation: keep the package scope
  tight, review the threshold, and require a written justification with each
  exclusion.
- Field-coverage reflection can be brittle as the schema nests; structural and
  identity fields live on an explicit allowlist, and a failure is the prompt
  to add a scenario, not to relax the test.
- Goldens can ossify a wrong number the first time they are written; only
  accepting golden changes alongside an oracle case or spec change keeps them
  subordinate to truth.
- Invariant checks encode assumptions (for example, monotonicity) that could
  themselves be wrong. If one fails, the engine's behavior is what is wrong;
  the response is to fix the engine or solver, never to delete the check.
- Per-PR CI time could push contributors to bypass gates; quick gates run per
  PR and the full soak runs nightly so a bypass is never the easy path.

## Pros and Cons of the Options

### Layered executable gates

- Good: covers value truth, behavioral invariants, drift, and suite growth;
  self-extending through data; fits the existing CI and hew workflow.
- Bad: most moving parts; slower CI; tooling and golden maintenance costs.

### Status quo

- Good: zero new cost; simple.
- Bad: coverage cannot detect a wrong assertion; correctness rests on review
  attention that neither humans nor agents can guarantee over time.

### Oracle-only

- Good: directly pins real values and catches systematic tax-law misreadings.
- Bad: each new case needs a human to derive truth; misses invariants,
  determinism, and the solver contract; does not force tests to discriminate.

### Invariant-only

- Good: cheap, generated coverage of a huge input space.
- Bad: a formula can be wrong in every case while all invariants hold; gives
  no anchor to published values.

### Golden-only

- Good: instantly catches unintended numeric drift, refactors included.
- Bad: enshrines whatever the engine does now; without an oracle it will
  happily freeze a bug for years.

### Formal methods

- Good: strongest possible guarantee; could prove monotonicity and bracket
  arithmetic rather than testing samples.
- Bad: high setup and maintenance cost, specialized skills, and the ±$1 gate
  already defines an acceptable tolerance that proofs are not needed to meet.

## Implementation Notes

- Turn `internal/validate` into a scenario registry: each entry is a named
  config, expected values with a source citation, and the years they apply.
  One harness per entry runs the oracle assertions (±$1), the invariant
  battery, and the golden comparison. The existing §7 cases become the first
  entries.
- Invariant battery, applied to every scenario and to generated configs
  (`pgregory.net/rapid` or equivalent): determinism (two runs identical),
  conservation (begin plus returns minus withdrawals minus end equals zero
  within cent-rounding), non-negative balances and ACB, no NaN/Inf, and the
  solver contract — for `r = SolveGross(target, capacity, net)`, either
  `net(r) >= target` or `r == capacity`.
- Field coverage: a reflection test asserting every leaf field of the config
  schema is set by at least one scenario; structural identity fields are
  allowlisted explicitly. Adding a config knob without a scenario fails CI.
- Goldens live under `testdata/golden/`, one per scenario. CI fails if a
  golden changed while neither the governing spec nor a validation case
  changed in the same PR; pure refactors must leave goldens byte-identical.
- Mutation gate: gremlins scoped to the tax, projection, and constants
  packages with an efficacy threshold; a quick run per PR, a full run nightly.
  The coverage gate remains a floor, not evidence.
- Fuzzing at the input boundary only: `FuzzParse` in `internal/config` seeded
  from `household.example.yaml`, time-boxed per PR, longer nightly; only
  regression inputs are committed under `testdata/fuzz`.
- Nightly workflow: full mutation, long fuzz, and an invariant soak; on
  failure it opens or updates an issue automatically (hew/gh), deduplicated by
  failure signature.
- Required checks: every gate above is a required status check on main, so a
  red gate blocks merge without anyone deciding to care.
- Invariants future changes must respect: expected values are added only with
  an external source; no golden changes without a spec or validation-case
  change; new config fields arrive with a scenario; the solver's monotonicity
  is a tested premise.
- Revisit this decision if CI time becomes a real bottleneck, if mutation
  exclusions creep, or if an external auditor or penny-exactness requirement
  appears — the latter is already ADR-0004's revisit condition.

## References

- Ontario Retirement Simulator Spec §7 (validation cases, ±$1 gate), §9
  (indexation governance and annual re-verification)
- ADR-0001 (Go; `go test` ergonomics, Phase 4 Monte Carlo), ADR-0002 (nominal
  engine; determinism), ADR-0003 (constants provenance), ADR-0004 (float64 and
  cent-rounding boundaries)
- `.github/workflows/ci.yml` — current gates: test, vet, gofmt, >90% coverage,
  govulncheck
- `internal/projection/solve.go` — the monotonicity premise the solver
  contract tests