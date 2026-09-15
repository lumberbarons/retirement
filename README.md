# retirement

Deterministic projection engine for an Ontario household's retirement finances,
implementing [`ontario-retirement-simulator-spec.md`](ontario-retirement-simulator-spec.md):
the annual order of operations, the dated 2026 constants and their index bases,
and the validation cases the engine is gated on.

## Modelling conventions

### OAS annualization

OAS is re-indexed every quarter (January, April, July, and October), so a
year's pension is not twelve times any single quarter's rate. The engine stores
the published monthly rate for each quarterly review in the dated table and sums
the twelve months. For 2026 it carries the most recent published rate through
quarters the rate card does not separately state: the Jan–Mar rates ($742.31 at
65–74, $816.54 at 75+) through April–June, and the Jul–Sep rates ($751.97 and
$827.17) through October–December. The year's twelve-month total is $8,965.68 at
65–74 and $9,862.26 at 75+. From the year after the newest dated row, the
four-quarter total grows by one annual CPI factor per year — the documented
convention for quarterly indexation in an annual model. The quarterly shape is
not re-applied inside each future year, so the total is never double-indexed.
The ESDC full-clawback ceilings are published against twelve times the January
rate, so at the ceiling the recovery leaves a few tens of dollars of the larger
annualized pension unclawed.

### OAS recovery timing

The recovery tax is administered on a July-to-June payment period against the
*prior* calendar year's income: Service Canada withholds it from a different
tax year than the income that triggered it. The engine takes a same-year
approximation instead — `recovery = min(OAS received, 0.15 × max(0, net income −
threshold))`, computed from the same year's income and charged in that year's
cash flow. Relative to the real cycle this shifts each year's recovery by up to
a year (a rising income is recovered sooner, a falling one later) without
changing the 15% rate or the income base.
