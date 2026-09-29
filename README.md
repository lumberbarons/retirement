# retirement

Deterministic projection engine for an Ontario household's retirement finances,
implementing [`ontario-retirement-simulator-spec.md`](ontario-retirement-simulator-spec.md):
the annual order of operations, the dated 2026 constants and their index bases,
and the validation cases the engine is gated on.

## Modelling conventions

### OAS annualization

OAS is re-indexed every quarter (January, April, July, and October), so a
year's pension is not twelve times any single quarter's rate. The engine stores
the applicable monthly rate for each quarterly review in the dated table and
sums the twelve months. The 2026 monthly rates are $742.31/$816.54 for January–March,
$743.05/$817.36 for April–June, $751.97/$827.17 for July–September, and
$762.50/$838.75 for October–December (65–74/75+). The October rates apply
ESDC's announced 1.4% adjustment to the July rates. The year's twelve-month
total is therefore $8,999.49 at 65–74 and $9,899.46 at 75+. From the year after
the newest dated row, the four-quarter total grows by one annual CPI factor per
year — the documented convention for quarterly indexation in an annual model.
The quarterly shape is not re-applied inside each future year, so the total is
never double-indexed.

### OAS recovery timing

The recovery tax is administered on a July-to-June payment period against the
*prior* calendar year's income: Service Canada withholds it from a different
tax year than the income that triggered it. The engine takes a same-year
approximation instead — `recovery = min(OAS received, 0.15 × max(0, net income −
threshold))`, computed from the same year's income and charged in that year's
cash flow. Relative to the real cycle this shifts each year's recovery by up to
a year (a rising income is recovered sooner, a falling one later) without
changing the 15% rate or the income base.

The published ESDC recovery-range ceilings ($154,708 at 65–74 and $160,647 at
75+) reconcile with twelve January payments and remain external reference
figures for the real July-to-June cycle. They are not the full-recovery points
for the larger four-quarter pension under the same-year approximation. With a
$95,323 threshold, the model fully recovers the 2026 annual pension at
`threshold + annual OAS / 0.15`: $155,319.60 at 65–74 and $161,319.40 at 75+.
