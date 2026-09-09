# Implementation Specification: Retirement Simulator for a Married Ontario Couple (2026 Base Year)

**Version 3.** Adds the spending-target model (§6), a zero-bequest configuration (§5.6), inter-spousal income-splitting vehicles (§1.4), and locked-in accounts (§1.5) — then **fact-checked against CRA and ESDC primary sources**. Three corrections were made (Ontario brackets, Ontario surtax thresholds, OAS full-clawback ceilings); see the verification table in §10.

---

## 0. SCOPE AND HOUSEHOLD PROFILE

Two spouses, currently mid-40s, Canadian residents for tax purposes in **Ontario** for the entire projection. Model from the current year through **second death** (potentially 50+ years).

**Household configuration flags that change the optimization:**

```
HOUSEHOLD = {
  "children": False,            # no bequest motive; see §5.6
  "target_terminal_estate": 0,  # real dollars at second death
  "province": "ON",
  "residency_years_at_65": 40,  # full OAS for both
}
```

Because `children == False`:
- The **CPP child-rearing drop-out provision is not applicable** — omit it from the CPP calculation entirely (§2.1).
- The **RESP is out of scope**. RDSP and FHSA are also out of scope; IPPs are out of scope unless a spouse is incorporated.
- The estate/legacy logic in §5.6 replaces the conventional bequest-preservation heuristics.

---

## 1. ACCOUNT MECHANICS

### 1.1 TFSA

```
TFSA_ANNUAL_LIMIT = {2009:5000, 2010:5000, 2011:5000, 2012:5000,
  2013:5500, 2014:5500, 2015:10000, 2016:5500, 2017:5500, 2018:5500,
  2019:6000, 2020:6000, 2021:6000, 2022:6000, 2023:6500,
  2024:7000, 2025:7000, 2026:7000}
```

- Cumulative room 2009→2026 (continuously eligible, never contributed) = **$109,000**.
- **Indexation:** annual dollar limit indexed to CPI then **rounded to nearest $500**. Model as `next_limit = round_to_500(7000 × cumulative_CPI_factor)`. At ~2% inflation the next step to $7,500 lands around 2027–2028; thereafter increment $500 roughly every 3–4 years.
- **Room formula each Jan 1:** `room = unused_prior_room + current_year_limit + prior_year_withdrawals − contributions_made`.
- **Recontribution rule (off-by-one hazard):** a withdrawal is added back to room on **January 1 of the year AFTER** the withdrawal. A 2026 withdrawal restores room Jan 1, 2027.
- **Tax treatment:** growth and withdrawals tax-free; withdrawals do **not** enter net income → no effect on OAS clawback, GIS, or age-amount phase-outs. This is why the TFSA is the ideal clawback-management buffer.
- **At death — successor holder:** name the spouse **successor holder** (not merely beneficiary). The TFSA continues as the survivor's own TFSA with **no impact on the survivor's own contribution room**, no loss of tax-sheltered status. (A beneficiary designation instead permits a tax-free "exempt contribution" rollover up to date-of-death value, but post-death growth is taxable.) **Model:** on first death, merge the deceased's TFSA into the survivor's TFSA at full value, tax-free.
- **Gifting to a spouse's TFSA:** see §1.4 — attribution does not apply, making this the single easiest splitting tool.

### 1.2 RRSP / RRIF

**Contribution room:**

```
rrsp_new_room(year) = min(0.18 × earned_income(year−1), RRSP_DOLLAR_LIMIT[year]) − PA(year−1)
rrsp_room(year)     = rrsp_new_room(year) + unused_carryforward
RRSP_DOLLAR_LIMIT = {2025:32490, 2026:33810, 2027:35390}
```

Indexed to **average wage growth** (not CPI) — grows faster than TFSA/tax brackets; project ~3–4%/yr. Income needed to hit the 2026 cap: $33,810 / 0.18 = **$187,833**. Unused room carries forward indefinitely. $2,000 lifetime over-contribution cushion.

**Spousal RRSP:**
- Contributions go to a plan **owned by (annuitant = ) the lower-income spouse**; the **contributor** takes the deduction against their own room.
- **3-year attribution rule:** if the annuitant withdraws in the year of a spousal contribution or the **two calendar years following**, the withdrawal is attributed back to and taxed in the **contributor's** hands. Model: tag spousal contributions by year; on withdrawal, attribute `min(withdrawal, contributions_in_3yr_window)` to the contributor.
- **Contribution age limit follows the ANNUITANT, not the contributor.** A contributor over 71 who has already collapsed their own RRSP may still contribute to a spousal RRSP until **Dec 31 of the year the SPOUSE turns 71**, provided the contributor has room. For an age-gapped couple this is several extra years of deductions — model it explicitly rather than cutting off contributions at the contributor's 71st birthday.
- **Spousal RRIF carve-out:** once converted, the **minimum** withdrawal is **exempt** from the 3-year attribution rule. Only amounts **above the minimum** attribute back to the contributor. Failing to code this carve-out produces spurious attribution through the entire 70s.

**Age-71 conversion:** RRSP must be converted to a RRIF (or annuity) by **Dec 31 of the year the holder turns 71**. No RRIF minimum is required in the year the RRIF is opened; minimums start the following year.

**RRIF minimum factors (Income Tax Regulations 7308, post-1992 RRIFs):**

```
# Ages ≤70: factor = 1 / (90 − age)  → e.g., 65:4.00%, 70:5.00%
RRIF_FACTOR = {71:0.0528, 72:0.0540, 73:0.0553, 74:0.0567, 75:0.0582,
  76:0.0598, 77:0.0617, 78:0.0636, 79:0.0658, 80:0.0682, 81:0.0708,
  82:0.0738, 83:0.0771, 84:0.0808, 85:0.0851, 86:0.0899, 87:0.0955,
  88:0.1021, 89:0.1099, 90:0.1192, 91:0.1306, 92:0.1449, 93:0.1634,
  94:0.1879, 95:0.2000}  # 95+ = 20.00% flat
```

- **Minimum = RRIF fair market value on January 1 × factor for age at start of year.** Factors are fixed in tax law; **never indexed**.
- **Younger-spouse election:** at RRIF setup the holder may irrevocably elect to base minimums on the **younger spouse's age**, lowering the factor. Permanent, chosen at setup. Model as a per-account boolean.
- **Withholding on amounts ABOVE the minimum** (none on the minimum itself): 10% ≤$5,000; 20% >$5,000–$15,000; 30% >$15,000 (outside Quebec). Withholding is a prepayment, not final tax.
- **Full inclusion:** every RRSP/RRIF dollar withdrawn is fully taxable as ordinary income.
- **Rollover to spouse at death:** rolls tax-deferred to the surviving spouse (named beneficiary / successor annuitant) — no income inclusion on first death. On **second death** the entire remaining RRIF is included as income in the year of death (§5.5).

### 1.3 Non-Registered / Taxable Accounts

- **ACB tracking** per security; gain = proceeds − ACB. Reinvested distributions increase ACB; return-of-capital distributions reduce ACB.
- **Capital gains inclusion rate = 50%** (the proposed 66.67% rate was deferred Jan 31, 2025, then **cancelled March 21, 2025**; never in force). Taxable gain = 0.50 × net gain. Top Ontario combined rate on gains ≈ **26.76%**.
- **Canadian eligible dividends:** gross-up **38%**; federal DTC **15.0198%** of grossed-up amount; **Ontario DTC 10.0%** of grossed-up amount (Ontario 2026 Budget revises this for 2027+).
- **Canadian non-eligible dividends:** gross-up **15%**; federal DTC **9.0301%**; Ontario DTC **2.9863%**.
- **Surtax interaction:** apply the DTC to reduce basic Ontario tax *before* the surtax calculation. The gross-up inflates net income and therefore feeds the OAS clawback even though the DTC reduces tax payable — a key retiree trap. Top Ontario 2026 rates: ~39.34% (eligible) vs ~47.74% (non-eligible).
- **Foreign dividends & interest:** fully taxable at marginal rate, no gross-up, no DTC.
- **Foreign withholding + foreign tax credit:** US treaty withholding ~15% on dividends; claim FTC up to Canadian tax on that income. US withholding on dividends in an **RRSP is exempt** under the treaty but **not recoverable in a TFSA or non-registered account**.
- **Deemed disposition at death** at FMV — except property passing to a spouse, which rolls at ACB, deferring the gain to second death.
- **Asset location:** interest/foreign income → shelter in RRSP/TFSA; Canadian eligible dividends and capital gains most efficient in non-registered; TFSA best for highest-growth assets, but avoid US dividend payers in a TFSA.

### 1.4 Inter-Spousal Splitting Vehicles (accumulation-phase)

These change the **starting balances each spouse brings into decumulation**, which in turn determines how much annual splitting is needed later. Architecturally they belong in the accumulation engine, distinct from the annual in-retirement elections in §4.6 and §2.1.

**(a) Spousal RRSP** — see §1.2.

**(b) Prescribed rate loan.** Higher earner lends non-registered capital to the lower earner under a written loan agreement at not less than the CRA prescribed rate; investment income above that rate is taxed in the **borrower's** hands rather than attributing back.

```
PRESCRIBED_RATE = 0.03   # Q4 2026; set quarterly, rounded up to nearest whole %
                         # locked in for the LIFE of the loan at inception
INTEREST_DUE_BY = "January 30"   # of the following year — hard deadline
```

- The rate is **locked at inception** and does not change with later CRA announcements. A loan struck at 3% stays at 3% permanently. (Refinancing to a lower rate generally requires repaying the original loan, which may trigger dispositions.)
- **Interest must actually be paid by January 30** of the following year. Miss it and the attribution rules apply **for that year and every subsequent year** — the strategy is permanently dead, not merely suspended.
- **Model:** track `loan_principal` and `locked_rate`. Each year: add `principal × locked_rate` as **interest income to the lender** and as a **carrying-charge deduction for the borrower**; assign all portfolio income and gains on the loaned capital to the borrower. Net benefit ≈ `(portfolio_return − locked_rate) × principal × (lender_MTR − borrower_MTR)`.
- Only worthwhile when the return-minus-3% spread and the bracket gap are both meaningful. Include an on/off flag and a sensitivity toggle.

**(c) Gifting to a spouse's TFSA.** The attribution rules are switched off for TFSA contributions — the higher earner may fund the lower earner's TFSA outright with **no attribution** on anything earned inside. **Default behaviour in the accumulation engine: always fill both TFSAs to the annual limit regardless of whose income funds them.**

**(d) Expense shifting.** Higher earner pays all household costs; lower earner invests their own after-tax income in a non-registered account. No attribution — it is their own money. Model as a redirection of after-tax cash flow; compounds into a materially larger low-income-spouse portfolio over 20 years.

**(e) Second-generation income.** Income earned *on* previously attributed income is **not itself attributed**. Minor in magnitude; include a note so the engine does not attribute in perpetuity. Acceptable to approximate.

### 1.5 Locked-In Accounts (LIRA / LIF) — required if a commuted value is taken

If either spouse elects a **commuted value** instead of the DB pension, the funds go to a **LIRA**, later converted to a **LIF**. This is a distinct account type from RRSP/RRIF:

- LIF has **both a minimum** (same RRIF factors, §1.2) **and a provincial MAXIMUM** annual withdrawal. The Ontario maximum is the greater of the previous year's investment earnings or `F × Jan-1 balance`, where F is an age-based factor derived from a prescribed long-term Government of Canada bond rate (CANSIM series), published annually by FSRA.
- **Ontario 50% unlocking:** a one-time opportunity to transfer up to 50% of the amount to an RRSP/RRIF within **60 days** of the transfer into a New LIF. Also relevant: small-balance and financial-hardship unlocking provisions.
- **Model:** `LIF` account type with `min_withdrawal` and `max_withdrawal` bounds; the discretionary-withdrawal solver must clamp within both. Any RRSP-meltdown logic must respect the LIF **maximum**, which can materially constrain the strategy.
- **Verify current FSRA rules and the annual F-factor table at build time** — Ontario unlocking rules have been subject to ongoing reform proposals.

---

## 2. GOVERNMENT BENEFITS

### 2.1 CPP

```
CPP_MAX_65 = 1507.65   # $/month, new benefit Jan 2026
CPP_AVG_65 = 925.35    # $/month, average new
YMPE_2026 = 74600; YAMPE_2026 = 85000; YBE = 3500
CPP_rate_base = 0.0595 (each, to YMPE); CPP2_rate = 0.04 (YMPE→YAMPE)
PRB_MAX_2026 = 54.69   # post-retirement benefit, $/month
DEATH_BENEFIT = 2500   # one-time lump sum
```

- **Calculation:** ≈25% (base) rising toward **33.33%** (enhanced) of average YMPE-indexed pensionable earnings over the contributory period (18 → start), after drop-outs. CPP2 adds a slice on the YMPE→YAMPE band. Full enhancement accrues only over a ~40-year career under enhanced rates; mature target maximum ≈ $20,000/yr.
- **Drop-out provisions:** **child-rearing drop-out — NOT APPLICABLE (no children); omit.** **Disability drop-out** — exclude CPP-disability months. **General low-earnings drop-out = 17%** of remaining months (~8 years at a 47-year contributory period); automatic; applies only if >120 months remain.
- **Actuarial adjustment:** **−0.6%/month before 65** (max −36% at 60); **+0.7%/month after 65** (max **+42% at 70**). Permanent. Age 60 → 0.640 × base; age 70 → 1.420 × base.
- **Pension sharing (assignment):** both spouses 60+; shares based on months lived together during the joint contributory period. The combined total is unchanged — it shifts taxable income to the lower earner. Form ISP1002. PRB is not shareable; stops on separation/divorce/death. **CPP is NOT eligible for T1032 pension splitting** — sharing is the only mechanism.
- **Survivor's pension (2026):** under 65 max **$803.54/mo** ($238.17 flat + ≈37.5% of the deceased's calculated retirement pension); 65+ max **$904.59/mo** (≈60% of the deceased's retirement pension, no flat portion).
- **Combined survivor + own retirement cap: $1,531.56/mo (2026).** Applied to the age-65 baseline, so deferring one's own CPP to 70 can lift actual receipts above it.
  `survivor_top_up = max(0, min(cap, own_retirement + computed_survivor) − own_retirement)`
  If both spouses had near-max CPP, the survivor receives little or no top-up.
- **Indexation:** CPP in pay → CPI each January (2026: **+2.0%**). YMPE/YAMPE → **average wage growth** (2026 YMPE +~4.6%).

### 2.2 OAS

```
OAS_MAX_65_74 = 742.31   # $/month, Jan–Mar 2026
OAS_MAX_75PLUS = 816.54  # $/month (includes permanent +10% at 75)
OAS_CLAWBACK_THRESHOLD_2026 = 95323   # net income, per person
OAS_RECOVERY_RATE = 0.15
```

- **Eligibility:** full pension at **40 years** of residence after 18; minimum **10 years**; partial = 1/40th per year. Both spouses assumed to qualify for full OAS.
- **Indexation:** **quarterly** to CPI (Jan/Apr/Jul/Oct). For annual modelling apply the annual CPI factor; a full-year total is not exactly 12× any single quarter.
- **Deferral:** +0.6%/month past 65, max **+36% at 70**.
- **Recovery tax:** `recovery = min(OAS_received, 0.15 × max(0, net_income − 95323))`. Income base = net income (line 23400 less certain deductions → line 23500); **net world income includes the OAS itself**.
- **Full-clawback ceilings — the two government sources disagree.** ESDC's quarterly rate card gives **$154,708 (65–74)** and **$160,647 (75+)**; CRA's recovery-tax page gives **$154,753** and **$160,696**, explicitly footnoted as *estimates* until finalized in Oct–Dec. **Use the ESDC figures** — they reconcile exactly with the arithmetic (`95,323 + 8,907.72/0.15 = 154,708`). Difference is ~$45, immaterial to planning but it will show up as a failing unit test if you pick the wrong one.

> **Annual OAS total ≠ 12 × the January amount.** OAS is re-indexed **quarterly**. The Jan–Mar 2026 rate is $742.31/mo, but by Jul–Sep 2026 it had risen to **$751.97** (65–74) and **$827.17** (75+). Summing 12 × the January figure understates the year. Either sum four quarters or apply a mid-year average; document the choice, because it feeds the clawback calculation.
- **Administration:** applied over a **July-to-June payment period based on the PRIOR calendar year's income**; Service Canada withholds from the monthly payment. A same-tax-year approximation is acceptable for planning — **document whichever convention you implement**.

### 2.3 GIS

2026 (Jan–Mar): single max **$1,108.74/mo**, income cut-off (single) **$22,488**; spouse of an OAS pensioner **$667.41/mo** each. Non-taxable, income-tested.

- Reduces roughly **$0.50 per $1** of other income (excluding OAS, and excluding the first $5,000 of employment income plus 50% of the next $10,000) → **effective marginal rates of 50%+** stacked on regular tax.
- Usually zero for this household during core retirement, but **evaluate every year** — it can reappear for a low-income single survivor in late life. Where GIS is in play the meltdown logic **reverses**: accelerate RRSP withdrawals *before* OAS/GIS begins.

---

## 3. DEFINED BENEFIT PENSION

- **Income formula (configurable):** final average earnings → `accrual_rate × years_of_service × final_average_earnings` (e.g. 2% × 30 × best-5-year average). Career average → sum of accrual-rate × earnings each year.
- **Bridge benefit:** extra amount from early retirement **to age 65**, then **ceases**. Model as a separate stream that stops at 65. Integrated/coordinated plans step the lifetime pension down at 65 to reflect CPP.
- **Early retirement reduction:** typically ~**5%/year** below the unreduced age; member usually must be 55+.
- **Unreduced ("Rule of 85"):** no age reduction once age + service ≥ 85 (or a stated age).
- **Indexation — dominates DB adequacy.** Parameterize: full CPI, partial (50–75% CPI, or CPI minus a margin), ad hoc, or none. A non-indexed pension loses roughly a third of its real value over 30 years at 2% inflation. Make this an explicit, prominent input.
- **Survivor option (Ontario PBA s.44):** default **60% joint-and-survivor**. Electing it **actuarially reduces the member's base pension** based on both ages; the couple may waive down to as low as 50% (Form 3) to raise the base pension. Model `member_pension = base × J&S_reduction_factor`; on the member's death the survivor receives `elected_% × member_pension` (bridge excluded).
- **Pension Adjustment (PA):** approximate `PA ≈ (9 × annual_accrued_benefit) − 600`; **subtract from next year's RRSP room** while an active DB member.
- **Commuted value alternative:** if modelled, route to LIRA/LIF per §1.5.

---

## 4. TAXATION

### 4.1 Federal brackets 2026

```
FED_BRACKETS_2026 = [(0,58523,0.14), (58523,117045,0.205),
  (117045,181440,0.26), (181440,258482,0.29), (258482,inf,0.33)]
```

Lowest rate cut 15%→14% (Bill C-4), full-year in 2026. Indexed to CPI annually (2026 factor 2.0%).

### 4.2 Ontario brackets 2026

```
ON_BRACKETS_2026 = [(0,53891,0.0505), (53891,107785,0.0915),
  (107785,150000,0.1116), (150000,220000,0.1216), (220000,inf,0.1316)]
```

**Verified against CRA "Current year tax rates and income brackets (2026)", canada.ca, modified 2026-06-25.** Lower two thresholds indexed 1.9% for 2026; the **$150,000 and $220,000 thresholds are frozen (not indexed)** — permanent bracket creep.

> **Correction from v2.** Earlier drafts used $52,886 / $105,775 — those are the **2025** thresholds. Many secondary sources (payroll blogs, tax-calculator sites) still publish 2025 or even 2024 Ontario figures under a "2026" heading. Do not source Ontario brackets from anything but CRA.

### 4.3 Ontario surtax (on basic Ontario tax, not income)

```
surtax = 0.20 × max(0, ON_tax − T1) + 0.36 × max(0, ON_tax − T2)
ON_SURTAX_T1_2026 = 5818
ON_SURTAX_T2_2026 = 7446
```

> **Correction from v2.** Earlier drafts used $5,710 / $7,307 — the **2025** thresholds. 2026 values are $5,818 / $7,446, indexed 1.9%.

Combined top surtax 56%, lifting Ontario's effective top rate from 13.16% to ~20.53% and the combined top marginal rate to **53.53%**.

`ON_tax` means **basic Ontario tax AFTER non-refundable credits**, not before. This ordering is not cosmetic — it moves a retiree between surtax bands and changes the marginal rate by several points (see §5.2).

### 4.4 Ontario Health Premium 2026

```
def ohp(ti):  # ti = taxable income
  if ti <= 20000: return 0
  if ti <= 36000: return min(300, 0.06*(ti-20000))
  if ti <= 48000: return min(450, 300 + 0.06*(ti-36000))
  if ti <= 72000: return min(600, 450 + 0.25*(ti-48000))
  if ti <= 200000: return min(750, 600 + 0.25*(ti-72000))
  return min(900, 750 + 0.25*(ti-200000))
```

$0 (≤$20,000) to **$900** (>$200,600). The $20,000 entry threshold is **never indexed**. Added to Ontario tax, separate from surtax.

### 4.5 Non-refundable credits (valued at the LOWEST rate: 14% federal, 5.05% Ontario)

```
BPA_fed_2026 = 16452  # phases to 14829 as taxable income goes 181440 → 258482
BPA_on_2026  = 12989
AGE_AMOUNT_fed_2026 = 9208   # 15% phase-out of net income > 46432; nil at 107819
AGE_AMOUNT_on_2026  = 6342   # 15% phase-out of net income > 47210; nil at 89490
PENSION_AMT_fed = 2000       # NOT indexed; credit = 14% × 2000 = $280
PENSION_AMT_on  = 1796       # indexed; credit ≈ $90.70
SPOUSAL_AMT_fed_2026 = 16452 # reduced $-for-$ by spouse net income
SPOUSAL_AMT_on_2026  = 11029 # reduced by spouse income over ~1103
CANADA_EMPLOYMENT_AMT_2026 = 1501
```

- **Age amount** requires 65+; phases out at 15% of net income over the threshold.
- **Pension income amount ($2,000 federal):** **DB/RPP pension income qualifies at any age. RRIF/annuity income qualifies only at 65+. RRSP withdrawals never qualify. CPP/OAS never qualify.** Implement a per-person, per-income-type eligibility flag keyed on age.
- **Credit ordering:** aggregate the credit bases, multiply by the lowest rate, subtract from tax otherwise payable, floor at zero. Spousal/age/pension amounts transfer to the spouse if unused.
- **Charitable donation credit:** federal first-$200 at the lowest rate (**verify: tied to the lowest bracket, which fell to 14% for 2026**), then 29%, with 33% to the extent of income in the top bracket; plus Ontario. Normal annual limit **75% of net income**, rising to **100% of net income in the year of death and the immediately preceding year** — material for §5.6.

### 4.6 Pension income splitting (Form T1032)

- **Eligible:** DB/RPP life annuity payments (any age); at **65+**, also RRIF/LIF/annuity payments. **Not eligible:** CPP, OAS, RRSP withdrawals, and RRIF income before 65.
- Transfer **up to 50%** of eligible pension income to the lower-income spouse. Annual, optional, joint election; deduction line 21000 / income line 11600; withheld tax reallocated. No money moves.
- **Model as an annual optimization:** find `s ∈ [0, 0.5]` minimizing **combined household tax + OAS clawback + OHP + credit clawbacks**. The objective is piecewise-linear in `s` — evaluate on a 1% grid or at bracket/clawback breakpoints. Solve **jointly** with CPP sharing and the RRIF/discretionary withdrawal decision.

### 4.7 Combined effective marginal rate curve

At each income level, sum: federal bracket rate + Ontario bracket rate + Ontario surtax increment (20%/36% of the Ontario marginal) + OHP ramp + **OAS recovery 15%** (65+, in the clawback zone) + **age-amount phase-out 15% × (14% + 5.05%)** where applicable. Inside the clawback zone this exceeds the 53.53% statutory top. The engine should compute this curve and use it to size withdrawals.

### 4.8 Other Ontario/federal seniors' credits

- **OSHPTG:** up to **$500**, income-tested, Form ON-BEN, age 64+ at Dec 31.
- **Ontario Trillium Benefit** = OEPTC + OSTC + NOEC. OEPTC senior max ≈ **$1,488**, reduced 2% of AFNI over **$36,309**. OSTC **$378/adult**, reduced 4% of AFNI over **$37,273**.
- **GST/HST credit**, **medical expense credit** (15% fed / 5.05% ON of expenses over the lesser of 3% of net income or a fixed floor — significant in late life), **Canada Caregiver Credit**.
- Mostly phase to zero in core retirement but **reappear for a low-income survivor** — evaluate annually rather than assuming zero.

---

## 5. WITHDRAWAL SEQUENCING & OPTIMIZATION

### 5.1 Baseline vs. optimized ordering

Naive rule: non-registered → RRSP/RRIF → TFSA. Frequently suboptimal.

### 5.2 RRSP meltdown / bracket filling

Deliberately draw RRSP/RRIF in the low-income window (retirement → ~71) to fill low brackets before CPP + OAS + forced RRIF minimums stack. Constrain by any LIF maximum (§1.5).

**Two natural stopping points, with the arithmetic — do not hard-code these, derive them:**

| Ceiling | Combined marginal rate | Why |
|---|---|---|
| **$95,323** (OAS clawback threshold, per person) | **29.65%** | Federal 20.5% + Ontario 9.15%. Basic Ontario tax here (~$6,513 before credits, ~$5,765 after BPA + pension amount) sits just **below** the $5,818 surtax T1 → no surtax. |
| **$107,785** (top of Ontario's 9.15% bracket) | **31.48%** | Federal 20.5% + Ontario 9.15% × 1.20. Credits pull basic Ontario tax into the **20%-only** surtax band (above $5,818, below $7,446) → Ontario marginal becomes 10.98%. |

Above $107,785 the Ontario rate steps to 11.16% and the second surtax tier (36%) engages, so the marginal rate jumps sharply. Before age 65 (no OAS) the $107,785 ceiling governs; from 65 the $95,323 ceiling governs, because crossing it adds the 15% recovery tax on top.

**Both figures are credit-sensitive.** They assume the BPA and the pension income amount are claimed and the Ontario age amount has already phased out (nil above $89,490). Change the credit profile and the surtax band shifts — which is exactly why §4.7 tells the engine to compute the marginal-rate curve rather than trust a table.

### 5.3 Deferring CPP and OAS to 70

CPP +42%, OAS +36%, both fully indexed and guaranteed. Breakeven ≈ age 82. See §5.6 — the standard objection to deferral is a bequest argument and does not apply here.

### 5.4 Splitting toolkit

Spousal RRSPs and prescribed rate loans (accumulation phase, §1.4); pension splitting and CPP sharing (annual elections in retirement, §4.6 / §2.1). Level taxable income across years; defend **two** OAS pensions.

### 5.5 Second death and first/second death asymmetry

- **Estate tax bomb:** at second death the **entire remaining RRIF is included as income in one year**, frequently at ~53.53%. Compute terminal tax explicitly and surface it in outputs.
- **Asymmetry at first death (must model):** household loses **one OAS**, may lose CPP (subject to the survivor cap), **loses the ability to income-split**, and moves from **two sets of brackets and credits to one**. The survivor's marginal rate rises sharply even as household income falls only modestly.

### 5.6 Zero-bequest configuration (`children == False`, `target_terminal_estate == 0`)

The success criterion changes from "terminal balance ≥ 0 at age 95" (an uncontrolled, usually large, implicit bequest) to **terminal real wealth ≈ target at second death**. Consequences the engine must implement:

**(a) Sustainable spending rises materially** — commonly 15–25% versus a "don't run out by 95" constraint. This is the single largest lever in the model, larger than any tax optimization.

**(b) Longevity insurance replaces the bequest cushion.** A large residual was silently hedging the long-life scenario. Replace it deliberately:
- **CPP/OAS deferral to 70 becomes materially stronger.** The "you might die at 78" objection is a bequest argument; with no legacy motive only the expensive outcome (living long) matters. Weight deferral accordingly in the optimizer.
- **Model an annuity / deferred annuity (ALDA) purchase option** funded from registered assets. Mortality credits accrue precisely because there is no one to leave the capital to.
- **Plan to a later age, or use probabilistic mortality.** A joint-last-survivor couple has a meaningful probability of one spouse reaching 97–100. Fixed-age-95 planning is riskier without a bequest buffer. Default the deterministic mode to **age 100** for at least one sensitivity run.

**(c) More aggressive RRSP meltdown.** The terminal RRIF inclusion still occurs, but preserving registered capital has no beneficiary rationale. Loosen any heuristic that protects registered balances for estate purposes.

**(d) The TFSA's role narrows.** It remains **last** in the withdrawal order, but purely as the clawback-invisible tax-management buffer — not as an estate asset. Drop estate-driven TFSA preservation logic.

**(e) Charitable donation as a terminal-tax tool.** The donation limit rises to **100% of net income in the year of death and the preceding year** (§4.5). A charitable bequest can offset most or all of the terminal RRIF inclusion. If any residual is destined for charity, model the credit rather than treating it as an afterthought. Expose `charitable_bequest_pct` as an input.

**(f) Long-term care reserve — required, not optional.** With no children there is no informal caregiving fallback, raising the probability of paid care, especially in the single-survivor period. Model as **its own stream**, not as part of the retirement smile's late uptick:
```
ltc_reserve = { "start_age": 85, "annual_real_cost": <input>,
                "probability": <input>, "duration_years": <input> }
```
Run it as both a deterministic step-up in spending and a stochastic shock in Monte Carlo mode.

---

## 6. SPENDING TARGET MODEL

### 6.1 Real → nominal conversion

The user states spending in **today's dollars**; the engine inflates it:

```
spend_nominal(year) = spend_today × (1 + i)^(year − base_year)
```

Applied across the **entire horizon**, not just to the retirement date. At 2.1% inflation, $80,000 today ≈ $109k at age 60, $121k at 65, $149k at 75, $184k at 85, **$226k at 95** — roughly 2.8×. Comparing a real spending target to a nominally-growing balance is the most common user-facing error these tools make.

### 6.2 Nominal engine, deflate at the reporting layer

**Work in nominal internally; deflate once, for display only.** A real-dollar engine appears simpler but breaks on everything that is not CPI-indexed — and in Canada that is a long list: non-indexed DB pensions, the frozen Ontario $150k/$220k brackets, the never-indexed $2,000 federal pension amount, the never-indexed $20,000 OHP floor, YMPE and the RRSP dollar limit (wage growth, not CPI). In a real-dollar engine each of these needs its own deflator — that is where the bugs breed.

**Index basis registry (every constant carries one):**

| Basis | Applies to |
|---|---|
| CPI | Tax brackets & credits, TFSA limit, OAS amount & clawback threshold, CPP in pay |
| Average wage growth | YMPE, YAMPE, RRSP dollar limit |
| Fixed / never indexed | RRIF factors, federal $2,000 pension amount, OHP $20,000 floor, Ontario $150k/$220k brackets |
| Plan-specific | DB pension indexation (§3) |
| User-set | Spending inflation (§6.5) |

### 6.3 The gross-vs-net solve (critical)

"How much do you want per year" means **after-tax spending**, but the engine controls **gross withdrawals**, and tax depends on the withdrawal. This is circular:

```
target_net = spend_nominal(year)
solve for gross_withdrawal such that:
    gross_income(gross_withdrawal) − total_tax(...) − oas_recovery(...) == target_net
```

Total tax has no closed form once surtax, OHP, OAS recovery, and age-amount phase-outs stack. **Solve numerically — bisection on gross withdrawal, tolerance $1, cap ~50 iterations.** It is monotonic (more gross always yields more net, even inside the clawback zone where the effective marginal rate exceeds 60%), so bisection is safe.

**Run this inside the annual loop, after the pension-splitting optimization**, since the split changes the tax and therefore the required gross.

The solve is really over the **withdrawal mix**, not a scalar: $10k from a TFSA nets $10k; $10k from a RRIF might net $6.5k; non-registered falls between depending on ACB. Have the sequencing strategy (§5) supply a **priority order** first, then bisect on the amount within that order.

### 6.4 Spending is not flat in real terms

Expose as configurable modes:

- **Retirement smile (recommended default):** high early "go-go" years, real decline through the 70s–80s, uptick late for health/care. Blanchett's work puts the middle-phase real decline at roughly **1%/yr below inflation**. Flat-real typically **overstates required capital by 10–15%**.
- **Lumpy items as separate streams:** vehicle replacement every ~8 years, roof, a travel budget running only the first ~12 years. Do not smear these into the base rate.
- **Survivor adjustment: ~70% of couple spending**, not 50% — housing and most fixed costs do not shrink. Pair with §5.5: spending drops ~30% while the survivor's marginal rate **rises**.
- **Long-term care:** separate stream per §5.6(f).

### 6.5 Spending inflation ≠ bracket indexation

Retiree baskets skew toward healthcare and property tax, historically running above headline CPI. Keep **two independent parameters**: `spending_inflation` and `tax_indexation_rate`. Given Ontario's frozen upper brackets and never-indexed OHP floor, divergence between the two compounds meaningfully over 50 years.

### 6.6 Two run modes

1. **Goal-seek / accumulation:** given a real spending target and retirement age, solve for the portfolio required at retirement and the annual savings needed. (The familiar consumer-calculator mode.)
2. **Cash-flow projection (primary mode for this household):** given actual balances and contributions, solve for the **maximum sustainable real spend**, or the probability a stated spend survives to second death.

Mode 2 implementation: **bisect on `spend_today`**, running the full projection at each trial value, scoring on `terminal_real_wealth >= target_terminal_estate` (deterministic) or `monte_carlo_success_rate >= 0.85` (stochastic). This produces "here's your number" without the user having to guess it first.

---

## 7. SIMULATION MODEL DESIGN

**Architecture:** annual time-step loop from the current year to max(both deaths).

**State objects:**
- `Person`: birth year, alive flag, CPP entitlement & start age, OAS start age, DB pension params, residence years, per-income-type credit eligibility.
- `Account`: type (TFSA / RRSP / RRIF / **LIF** / NonReg), owner, balance, ACB, Jan-1 snapshot, `younger_spouse_election` flag, `spousal_contributions_by_year` (for attribution).
- `Loan`: prescribed-rate loan principal, locked rate, lender, borrower.

**Order of operations within each year (fix this):**
1. Snapshot Jan-1 balances (for RRIF/LIF minimums and maximums).
2. Apply investment returns and inflation.
3. Mandatory income: DB pension, CPP, OAS, **RRIF minimum = Jan-1 balance × factor(age at Jan 1)**, prescribed-loan interest.
4. Discretionary withdrawals per the §5 strategy, clamped by any LIF maximum.
5. Compute each spouse's taxable income; run the **pension-splitting + CPP-sharing optimization**; apply spousal-RRSP attribution (with the spousal-RRIF minimum carve-out).
6. Federal tax, Ontario tax, surtax, OHP, credits, dividend gross-up/DTC, capital gains.
7. OAS recovery tax; GIS and income-tested credit clawbacks.
8. **Bisect gross withdrawals to hit `target_net` (§6.3)** — this feeds back into steps 4–7; iterate to convergence.
9. Roll balances forward; recompute TFSA/RRSP room for next Jan 1 (including prior-year TFSA withdrawals).
10. Handle death events: rollovers, loss of one OAS, CPP survivor cap, switch to single-filer brackets/credits, stop splitting and sharing, apply survivor spending factor (§6.4).

**Return modelling — three modes:** deterministic; Monte Carlo (report success probability); historical bootstrap. **Sequence-of-returns risk:** poor early-decumulation returns are far more damaging than the same returns later, because withdrawals crystallize losses. Deterministic mode hides this — with a zero-bequest, higher-spend plan the margin for error is thinner, so **Monte Carlo is not optional here**.

**FP Canada / Institute of Financial Planning 2026 Projection Assumption Guidelines** (long-term 10yr+, before fees, April 2026):

```
inflation             = 2.1%
salary/shelter growth = 3.1%
short_term (91-day)   = 2.4%
fixed_income          = 3.2%
canadian_equity       = 6.3%
us_equity             = 6.4%
intl_developed_equity = 6.6%
emerging_equity       = 7.5%
borrowing_rate        = 4.4%
```

Subtract investment fees; blend equity classes by portfolio weights.

**Longevity:** model **joint-and-last-survivor** survival. FP Canada 2026 uses **CPM2014 taken forward to 2026 via CPM Improvement Scale B**; alternatively Statistics Canada complete life tables. Offer fixed-age mode (default 95, plus a mandatory age-100 sensitivity per §5.6(b)) and probabilistic mode.

**Validation test cases:**
- RRIF minimum at 71 on $500k Jan-1 = $26,400 (5.28%); at 72 = 5.40%.
- OAS fully clawed back (65–74) at $154,708 net income (ESDC); 75+ at $160,647.
- Full OAS $742.31/mo (65–74), $816.54/mo (75+) at 40 years' residence.
- CPP at 60 = 0.640 × base; at 70 = 1.420 × base.
- $16,452 federal BPA → ~$2,303 federal credit at 14%.
- Combined survivor + own retirement CPP capped at $1,531.56/mo.
- Ontario surtax begins once basic Ontario tax (after credits) exceeds $5,818; the 36% tier at $7,446.
- Ontario basic tax on $107,785 taxable income, before credits = $7,652.80 (`53,891 × 5.05% + 53,894 × 9.15%`).
- Ontario Health Premium: $30,000 → $300; $37,000 → $360; $48,300 → $525; $72,300 → $675; $250,000 → $900.
- $80,000 real at 2.1% inflation → $121,000 nominal in 20 years.
- Gross-vs-net solve: a target net of $80,000 drawn wholly from a RRIF should converge to a gross figure whose computed net is within $1.
- Spousal RRIF minimum withdrawal within the 3-year window attributes **$0** to the contributor.

**Known edge cases / off-by-one errors:** RRIF minimum uses **Jan-1 balance and age at start of year**; no minimum in the RRIF's opening year; OAS clawback uses **prior-year income** on a July–June cycle; TFSA withdrawal restores room **next** Jan 1; prescribed-loan interest due **Jan 30** (not 31); partial first year of retirement (pro-rate); age-based eligibility keyed to age at year-end vs. specific month.

---

## 8. REFERENCES AND BENCHMARKS

- **Federal Canadian Retirement Income Calculator** (canada.ca) — CPP/OAS baseline.
- **Snap Projections**, **Adviice / PlanEasy** (Owen Winkelmolen), **Optiml**, **Cascades**, **PERC**, **RetireZest** — decumulation optimizers to benchmark logic against.
- **FP Canada Projection Assumption Guidelines** (fpcanada.ca) — annual assumption set; Institute of Financial Planning addendum documents the CPM2014 / Scale B mortality basis.
- **Decumulation writing:** Aaron Hector, Owen Winkelmolen, Ed Rempel, PWL Capital (Ben Felix), Mark McGrath.
- **Primary sources to hard-code from:** CRA (brackets, credits, ITR 7308 RRIF factors, dividend gross-up/DTC, **T4032-ON**, T1032, prescribed interest rates), ESDC/Service Canada (CPP/OAS/GIS rate card), Ontario.ca (health premium, OTB, OSHPTG), Ontario Pension Benefits Act / **FSRA** (DB survivor rules, LIF maximums and unlocking), CRA Income Tax Folios.

---

## 9. BUILD PLAN

1. **Phase 1 — deterministic engine.** Annual loop, all 2026 constants, both spouses, full federal + Ontario tax (brackets, surtax, OHP, credits, dividends, capital gains), CPP/OAS/GIS, DB pension, RRIF minimums, LIF min/max, death events with rollovers, and the §6.3 gross-vs-net solve. **Proceed only when every §7 validation case passes within ±$1.**
2. **Phase 2 — optimization layer.** Annual pension-splitting and CPP-sharing optimizers, RRSP-meltdown/bracket-filling, CPP/OAS start-age optimization, combined marginal-rate curve, and the §6.6 mode-2 bisection on `spend_today`.
3. **Phase 3 — accumulation-phase vehicles.** Spousal RRSP, prescribed rate loan, spousal TFSA gifting, expense shifting — each with an on/off flag so their effect on starting decumulation balances can be measured independently.
4. **Phase 4 — stochastic + longevity.** Monte Carlo and historical bootstrap with sequence-of-returns; joint-last-survivor mortality (CPM2014 + Scale B); the LTC shock; annuity/ALDA option. Report success probability and terminal (second-death) tax.
5. **Indexation governance.** Store every year-specific constant in a **dated table keyed by year with its index basis** (§6.2). Re-verify against CRA and the Service Canada rate card **every January**; flag any constant older than 12 months.

---

## 10. CAVEATS

- All dollar figures are **2026** and require **annual updating**.

**Verification status (checked against primary sources):**

| Item | Status |
|---|---|
| Federal brackets 2026 | **Verified** — CRA current-year page |
| Ontario brackets 2026 | **Corrected** — were 2025 values; CRA confirms $53,891 / $107,785 |
| Ontario surtax 2026 | **Corrected** — were 2025 values; now $5,818 / $7,446 |
| Ontario Health Premium function | **Verified** — reproduces all official plateaus and ramps |
| RRIF factors 71–95+ | **Verified** — all 25 values match ITR 7308 |
| OAS threshold $95,323 | **Verified** — canada.ca recovery-tax page |
| OAS full-clawback ceilings | **Corrected** — ESDC $154,708 / $160,647 preferred over CRA estimates |
| CPP $1,507.65 / YMPE $74,600 / YAMPE $85,000 | **Verified** — ESDC 2026 rate card |
| CPP survivor $803.54 / $904.59, combined cap $1,531.56 | **Verified** |
| TFSA $7,000, cumulative $109,000 | **Verified** — table sums to $109,000 |
| RRSP dollar limit $33,810 | **Verified** — CRA via Investment Executive |
| Prescribed rate 3% | **Verified** — CRA Q4 2026 |
| Capital gains inclusion 50% | **Verified** — increase cancelled March 2025 |
| Federal BPA $16,452 | **Verified** |
| Federal/Ontario age amount, pension amounts, spousal amounts | **Not independently verified** — internally consistent with 2.0%/1.9% indexation off 2025 values, but confirm against CRA T4032-ON / Schedule 1 before relying on them |
| Ontario eligible DTC 10.0%, non-eligible 2.9863% | **Not independently verified** — Ontario's 2026 Budget revises the credit for 2027+; confirm both the 2026 rate and the scheduled change |
| GIS amounts, OTB/OSHPTG thresholds | **Not independently verified** |

- **Source quality warning.** Several tax-calculator and payroll sites publish 2024 or 2025 Ontario figures under a "2026" heading — during this review, three separate sites gave three different surtax thresholds ($5,315, $5,554, $5,818) all labelled 2026. **Use CRA and ESDC only** for anything the engine depends on.
- **OAS clawback timing** (prior-year income, July–June cycle) is genuinely complex; a same-year approximation is acceptable if documented.
- **CPP benefit calculation** (drop-outs, YMPE-indexing of historical earnings, enhancement phase-in) should be **approximated** from each spouse's estimated entitlement (My Service Canada Account) scaled by start-age factors, rather than reconstructing full earnings histories. The **$1,531.56 combined survivor/retirement ceiling** is consistent with 2% indexation from 2025's $1,449.53 but should be confirmed against the primary Service Canada rate card.
- **The prescribed rate is set quarterly**; 3% (Q4 2026) is the rate for *new* loans struck now and locks for that loan's life. Re-verify before implementing.
- **Ontario LIF maximums and unlocking rules** change; verify the current F-factor table and unlocking provisions with FSRA.
- **DB pension formulas, indexation, bridge and survivor terms vary by plan** — expose as inputs, never hard-code.
- **Capital gains inclusion rate is 50% and confirmed for 2026**, but is a recurring political target — flag for annual re-verification.
- The **charitable donation first-$200 credit rate** follows the lowest federal bracket, which fell to 14% for 2026 — verify the exact interaction before relying on it in terminal-tax planning.
