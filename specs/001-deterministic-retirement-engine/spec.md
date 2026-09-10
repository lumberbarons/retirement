# Deterministic Retirement Engine

**Slug**: `001-deterministic-retirement-engine`
**Created**: 2026-09-09
**Status**: Accepted

## Description

A deterministic retirement projection engine for a two-spouse Ontario
household with no children and no bequest motive. From a single household
config file it projects both spouses' income, taxes, withdrawals, and account
balances year by year, from today through the second death, solving each year
so that spending nets a stated target expressed in today's dollars. It is the
first epic of the retirement calculator because every later capability —
optimization, accumulation vehicles, stochastic runs — is a layer on top of
this engine, and the governing implementation spec gates all further work on
its validation cases passing within ±$1.

## Prior Decisions

- [Ontario Retirement Simulator Spec §6.2](../../ontario-retirement-simulator-spec.md) — the engine works in nominal dollars and deflates once at the reporting layer; every year-specific constant carries its index basis (CPI, average wage, fixed, plan-specific, or user-set)
- [Ontario Retirement Simulator Spec §0 and §5.6](../../ontario-retirement-simulator-spec.md) — zero-bequest household (`children == False`, `target_terminal_estate == 0`): success means terminal real wealth ≈ 0 at second death; the CPP child-rearing drop-out, RESP, and bequest-preservation heuristics are omitted entirely
- [Ontario Retirement Simulator Spec §2.1 and §10](../../ontario-retirement-simulator-spec.md) — CPP is approximated from each spouse's stated entitlement scaled by start-age factors, never reconstructed from earnings history
- [Ontario Retirement Simulator Spec §2.2 and §10](../../ontario-retirement-simulator-spec.md) — a same-year OAS clawback approximation is acceptable if documented; the ESDC figures ($154,708 / $160,647 full-clawback ceilings) are authoritative
- [Ontario Retirement Simulator Spec §7](../../ontario-retirement-simulator-spec.md) — the annual order of operations is fixed as written there, and its validation cases within ±$1 are the gate this epic must pass

## Out of Scope

- The optimization layer: T1032 pension-splitting and CPP-sharing optimizers, RRSP-meltdown/bracket filling, CPP/OAS start-age optimization, and the §6.6 mode-2 bisection for maximum sustainable spend — the next epic. This epic accepts a user-set split fraction and sharing election (defaults off) as config inputs.
- §6.6 mode-1 goal-seek (solving for required savings), deferred with mode 2
- Accumulation-phase *splitting* vehicles — spousal RRSP funding elections, prescribed-rate loans, spousal TFSA gifting (§1.4(c)'s attribution-free default-fill rule), and expense shifting — a later epic; withdrawal-time attribution of accounts already declared spousal in config IS in scope. Plain, non-split accumulation mechanics — each spouse's earned income, their own RRSP/TFSA contributions, and PA accrual while an active DB member — ARE in scope; see Assumptions.
- LIRA/LIF mechanics — no commuted value is planned for this household. This narrows the governing spec's Phase 1 (§9), which lists "LIF min/max" as part of the gate; none of the governing spec's §7 validation cases exercise a LIF, so the ±$1 gate is still satisfiable without it. Revisit — and add the LIF min/max clamp back into this epic — if a commuted value becomes part of the plan.
- Stochastic modelling: Monte Carlo, historical bootstrap, sequence-of-returns reporting, probabilistic (CPM2014) mortality, the long-term-care *stochastic shock*, and the annuity/ALDA purchase option. The long-term-care reserve's deterministic spending step-up (§5.6(f)) is in scope as an ordinary lumpy stream (§6.4, T009) — only the Monte Carlo shock is deferred.
- Charitable bequest credit and donation modelling
- Foreign dividends, foreign withholding, and the foreign tax credit (§1.3) — no governing-spec validation case exercises them; revisit if the household holds foreign-income-generating non-registered assets
- Web UI — the report is CLI text plus CSV; a front end is a later epic
- Annual constants re-verification process — the dated-table layout enables it, the workflow itself is documented later

## Assumptions

- The engine and CLI are written in Go per the standing convention for new backends, with a web frontend as a later epic; this greenfield stack choice deserves its own ADR before implementation starts
- Household config is a single YAML file; a working example ships with the repo
- Each spouse's CPP is entered as the expected monthly amount at 65 (from My Service Canada Account) plus a chosen start age; the engine applies the actuarial factors, indexation, and survivor cap
- OAS recovery tax uses a same-year approximation (reality is prior-year income on a July–June cycle) and quarterly indexation is applied as one annual CPI factor — both conventions documented in the README, as the governing spec requires
- Deaths occur at fixed, user-set ages (default 95; an age-100 sensitivity run is a config edit away), per the governing spec's deterministic-mode default
- Deterministic returns default to the FP Canada 2026 Projection Assumption Guidelines and are user-overridable per asset class
- Both spouses have full OAS (40 years' residence at 65) and remain Ontario residents for the whole projection, per the household profile in the governing spec
- RRIF minimums use age at January 1; credit eligibility uses age at December 31 — fixing the convention the governing spec flags as an off-by-one hazard
- Spousal RRSPs may exist on day one (tagged with contribution years in config) even though making new spousal contributions is out of scope
- The household's pre-retirement working years are in scope, not just decumulation: each spouse's earned income to retirement is a config input, drives RRSP room accrual (T024) and, while a spouse is an active DB member, the pension adjustment (T020); each spouse's own-income RRSP/TFSA contributions follow simple config-set annual amounts. What stays out of scope is specifically the *splitting* vehicles built on top of that — spousal RRSP elections, prescribed-rate loans, TFSA gifting, expense shifting — not the underlying earned-income and contribution mechanics

## User Stories

### US1 — Configure the household (P1)

As a household planner, I want to describe both spouses, every account, benefit, pension, and our spending target in one config file so that the projection reflects our actual situation rather than a generic example

The config covers spouses (birth year, death age, earned income to
retirement, CPP entitlement at 65 and start age, OAS start age, DB pension
terms), accounts by type and owner
(TFSA, RRSP/RRIF with spousal tags and contribution years, non-registered
with ACB), and the spending target with its mode. Every dollar constant the
engine uses lives in dated tables keyed by year with their index basis — not
scattered literals — and a bad config fails with an error naming the first
offending field, not a stack trace.

### US2 — See the year-by-year projection to second death (P1)

As a household planner, I want a year-by-year projection of income, tax, withdrawals, and balances from today through our second death so that I can judge whether a stated lifestyle is affordable

The annual loop follows the fixed order of operations (Jan-1 snapshot →
returns and inflation → mandatory income → discretionary withdrawals → tax
and clawbacks → gross-net solve → roll-forward → death events), including a
pro-rated partial first retirement year. The report shows each year in both
nominal and today's dollars, and the summary reads terminal real wealth
against the $0 estate target — a large residual is a miss, not a windfall.

### US3 — Hit a real after-tax spending target (P1)

As a household planner, I want to state desired spending in today's dollars and have the engine solve the gross withdrawals that net exactly that so that I never have to guess pre-tax figures

Spending converts to nominal at its own inflation rate (independent of
tax-bracket indexation), shaped flat or as a retirement smile (roughly 1%/yr
real decline mid-phase), with optional one-off lumpy streams (travel budget,
roof, vehicle replacements, a long-term-care reserve step-up from a given
age) and a survivor spending factor of ~70% on first death. The solve bisects gross withdrawals within the baseline priority order
(non-registered → RRSP/RRIF → TFSA) to a $1 tolerance — monotonic, so it
converges even in the OAS clawback zone where the effective marginal rate
exceeds 60%.

### US4 — Trust the tax computation (P1)

As a household planner, I want federal and Ontario taxes computed under the actual 2026 rules — brackets, surtax, health premium, credits, dividends, capital gains — so that I can trust the projection for real decisions

The Ontario surtax is computed on basic Ontario tax after non-refundable
credits (the ordering moves a retiree between surtax bands), and the pension
income amount carries per-person, per-income-type eligibility flags (DB
qualifies at any age, RRIF at 65+, RRSP withdrawals and CPP/OAS never). Every
validation case in the governing spec runs as a named test in CI and must
land within ±$1 before any later epic starts.

### US5 — See government benefits per spouse (P1)

As a household planner, I want CPP, OAS, and GIS modelled for each spouse at our chosen start ages so that the projection shows what we will actually receive and what gets clawed back

CPP entitlement scales by start age (60 → 0.640×, 70 → 1.420×) and indexes
in pay; OAS defers up to +36% at 70 with a 15% recovery tax on net income
over the threshold, using the ESDC full-clawback ceilings ($154,708 at
65–74, $160,647 at 75+). GIS, and the smaller income-tested seniors' credits
(OSHPTG, the Ontario Trillium Benefit, the GST/HST credit, the medical
expense credit, and the Canada Caregiver Credit), are evaluated every year
even though they are usually zero for this household — they can reappear for
a low-income survivor in late life.

### US6 — Model the defined-benefit pensions (P1)

As a household planner, I want our DB pensions modelled with their actual terms — formula, bridge, indexation, survivor option — so that our biggest fixed income streams appear correctly

Accrual formula, bridge benefit (stops at 65), and indexation (full CPI,
partial, or none — a prominent input, since a non-indexed pension loses
roughly a third of its real value over 30 years) are all config, never
hard-coded. The 60% joint-and-survivor default applies a reduction factor to
the member's base pension and pays the elected percentage to the survivor
(bridge excluded); pension adjustments reduce next year's RRSP room while a
spouse is an active member.

### US7 — Have account mechanics tracked exactly (P2)

As a household planner, I want TFSA room, RRIF minimums, RRSP room, and spousal attribution computed to the letter so that small mechanical errors do not compound for decades

TFSA room on Jan 1 = unused + current limit + prior-year withdrawals −
contributions (a 2026 withdrawal restores room on Jan 1, 2027), tracked
against each spouse's own-income contributions — the spousal gifting
default-fill rule is a later epic's concern, not this one's. RRSP converts to
RRIF by Dec 31 of the year the holder
turns 71 (no minimum in the opening year), then minimum = Jan-1 balance ×
factor for age at Jan 1, with the younger-spouse election; withholding on
excess withdrawals is a prepayment against final tax; spousal withdrawals
attribute min(withdrawal, contributions in the 3-year window) to the
contributor, with the RRIF-minimum carve-out.

### US8 — Survive the first death and settle the second (P2)

As a household planner, I want first-death rollovers and survivor adjustments, and the second-death terminal tax, computed explicitly so that the plan holds together through widowhood and at the end

At first death the TFSAs merge via successor holder with no room impact, the
RRIF rolls over tax-deferred, the household loses one OAS, CPP survivor
benefits apply under the combined cap ($1,531.56/mo — near-max earners get
little or no top-up), brackets and credits go single, splitting stops, and
spending drops to the survivor factor. At second death the entire remaining
RRIF is included in the death-year return (often ~53.53%), non-registered
assets are deemed disposed at FMV, and the summary surfaces terminal tax and
estate value against the $0 target.

## Tasks

### US1

- [ ] T001 Scaffold the Go module and `retire` CLI entry point — `cmd/retire/main.go`
- [ ] T002 Define the household config schema (spouses incl. earned income to retirement, accounts, benefits, pensions, spending) — `internal/config/schema.go`
- [ ] T003 Load and validate config with field-level errors; ship a working example household — `internal/config/load.go`
- [ ] T004 Store every year-specific constant as dated tables keyed by year with index basis, plus the FP Canada 2026 default return/inflation assumptions — `internal/constants/tables.go`

### US2

- [ ] T005 Implement Person/Account state objects and Jan-1 snapshots — `internal/projection/state.go`
- [ ] T006 Implement the annual loop in the fixed order of operations, including a pro-rated partial first retirement year — `internal/projection/year.go`
- [ ] T007 Emit the year-by-year report (table and CSV, nominal and today's dollars, summary against the $0 estate target) — `internal/report/report.go`

### US3

- [ ] T008 Implement real→nominal conversion with spending inflation independent of tax indexation — `internal/projection/spending.go`
- [ ] T009 Implement flat and retirement-smile spending modes, lumpy one-off streams, and the survivor spending factor — `internal/projection/spending.go`
- [ ] T010 Implement the gross-vs-net bisection over withdrawals in the baseline priority order (non-registered → RRSP/RRIF → TFSA) — `internal/projection/solve.go`

### US4

- [ ] T011 Implement federal and Ontario bracket engines indexed from the 2026 base — `internal/tax/brackets.go`
- [ ] T012 Implement the Ontario surtax on basic tax after credits, the health premium, and non-refundable credits with per-type pension-amount eligibility — `internal/tax/ontario.go`
- [ ] T013 Implement eligible/non-eligible dividend gross-up and DTCs, and 50% capital-gains inclusion on ACB-tracked gains — `internal/tax/income.go`
- [ ] T014 Accept a user-set T1032 pension-split fraction per year (default 0) — `internal/tax/splitting.go`
- [ ] T015 Port every governing-spec validation case into a named Go test suite, ±$1, wired into CI — `internal/validate/cases_test.go`

### US5

- [ ] T016 Implement CPP by entitlement × start-age factor, CPI in-pay indexation, survivor benefit under the combined cap, and user-set sharing — `internal/benefits/cpp.go`
- [ ] T017 Implement OAS with deferral, recovery tax, and the ESDC full-clawback ceilings — `internal/benefits/oas.go`
- [ ] T018 Implement the annual GIS evaluation — `internal/benefits/gis.go`
- [ ] T019 Implement the annual evaluation of the smaller income-tested seniors' credits (OSHPTG, Ontario Trillium Benefit, GST/HST credit, medical expense credit, Canada Caregiver Credit) — `internal/benefits/seniors_credits.go`

### US6

- [ ] T020 Implement the DB pension stream (formula, bridge-to-65, indexation, J&S reduction, survivor percentage) and the PA that reduces RRSP room — `internal/pension/db.go`

### US7

- [ ] T021 Implement the TFSA room engine with Jan-1 prior-year-withdrawal recontribution and each spouse's own-income contributions — not the spousal gifting default-fill rule, deferred per Out of Scope — `internal/accounts/tfsa.go`
- [ ] T022 Implement RRSP→RRIF conversion at 71, Jan-1 minimum factors, younger-spouse election, and withholding as prepayment — `internal/accounts/rrif.go`
- [ ] T023 Implement spousal-RRSP attribution with the 3-year window and RRIF-minimum carve-out — `internal/accounts/attribution.go`
- [ ] T024 Implement RRSP room accrual from earned income (18% of earned income, dollar limit, carry-forward) and each spouse's own-income contributions during working years — `internal/accounts/rrsp.go`

### US8

- [ ] T025 Implement first-death events: TFSA successor-holder merge, RRIF rollover, one OAS lost, CPP survivor under the cap, single brackets/credits, splitting stopped, survivor spending factor — `internal/projection/death.go`
- [ ] T026 Implement the second-death terminal year: full RRIF inclusion, deemed disposition at FMV, terminal tax and estate value in the summary — `internal/projection/terminal.go`

## Done When

- [ ] `retire project --config household.example.yaml` completes a projection from the current year to the second death and writes a year-by-year table and CSV in both nominal and today's dollars
- [ ] Every validation case in the governing spec runs as a named test in CI and lands within ±$1, including the gross-vs-net solve, the RRIF minimums, the OAS clawback ceilings, and the spousal-RRIF minimum attributing $0 to the contributor
- [ ] A TFSA withdrawal in a projection year restores room on January 1 of the following year, shown by a unit test
- [ ] A run in which a spouse dies shows one OAS ceasing, CPP survivor applied under the combined cap, single brackets and credits, splitting stopped, spending at the survivor factor, and the TFSA merge adding no contribution room
- [ ] The terminal summary reports second-death tax including the full remaining RRIF inclusion, and terminal real wealth against the $0 estate target
- [ ] GIS and the other income-tested seniors' credits (OSHPTG, Ontario Trillium Benefit, GST/HST credit, medical expense credit, Canada Caregiver Credit) are evaluated every projection year rather than assumed zero, shown by a unit test where a low-income survivor year brings one or more back above zero
- [ ] An invalid config fails with an error naming the offending field, and the example household loads clean
- [ ] No 2026 dollar constant appears outside the dated tables in `internal/constants`, except in tests
- [ ] The OAS clawback timing approximation and the quarterly-indexation treatment are documented in the README
