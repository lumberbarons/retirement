package benefits

import (
	"math"
	"testing"
)

// withinDollarMonthly compares an annual GIS result against the rate card's
// monthly amount: the modelled annual reduction and the card's quarterly
// monthly figures agree to under a dollar at the published anchors.
func withinDollarMonthly(t *testing.T, got, wantMonthly float64) {
	t.Helper()
	if math.Abs(got/12-wantMonthly) > 1 {
		t.Fatalf("got %v/month, want %v within $1/month", got/12, wantMonthly)
	}
}

// TestGIS_ZeroWithoutOASReceipt covers the Done-when item that GIS is never
// paid without OAS eligibility and receipt: the pension must be in pay before
// the supplement can attach.
func TestGIS_ZeroWithoutOASReceipt(t *testing.T) {
	for _, single := range []bool{true, false} {
		got, err := GISAnnual(GISInput{
			Year: 2026, Forward: forward, Single: single, OASReceived: false,
		})
		if err != nil {
			t.Fatalf("GISAnnual: %v", err)
		}
		if got != 0 {
			t.Fatalf("GIS without OAS receipt (single=%v) = %v, want 0", single, got)
		}
	}
}

// TestGIS_SingleScheduleMatchesPublishedRateCard covers the Done-when item
// that the benefit declines on the implemented schedule without an artificial
// cut-off cliff. The anchors are the ESDC January-March 2026 single table
// (yearly income excluding OAS and GIS).
func TestGIS_SingleScheduleMatchesPublishedRateCard(t *testing.T) {
	single := func(income float64) float64 {
		t.Helper()
		got, err := GISAnnual(GISInput{
			Year: 2026, Forward: forward, Single: true, OASReceived: true, Income: income,
		})
		if err != nil {
			t.Fatalf("GISAnnual: %v", err)
		}
		return got
	}

	withinDollar(t, single(0), 12*1108.74)
	// The first $2,040 of income reduces the supplement at $0.50 per $1.
	withinDollar(t, single(2040), 12*1108.74-0.5*2040)
	withinDollar(t, single(1008), 12*1066.74)
	// The middle band reduces at $0.75 per $1, then the top band returns to
	// $0.50 and reaches nil at the published cut-off.
	withinDollarMonthly(t, single(6000), 775.74)
	withinDollarMonthly(t, single(10272), 509.00)
	withinDollarMonthly(t, single(22464), 1.00)
	if got := single(22488); got != 0 {
		t.Fatalf("GIS at the published cut-off = %v, want 0", got)
	}
	if got := single(30000); got != 0 {
		t.Fatalf("GIS above the published cut-off = %v, want 0", got)
	}
}

// TestGIS_SpouseOfPensionerUsesCombinedIncome covers the Done-when item that a
// couple's GIS uses the applicable combined household income test and
// recipient category: each spouse of a pensioner is tested on the couple's
// combined income, halved to the spouse-of-pensioner rate.
func TestGIS_SpouseOfPensionerUsesCombinedIncome(t *testing.T) {
	combined := func(income float64) float64 {
		t.Helper()
		got, err := GISAnnual(GISInput{
			Year: 2026, Forward: forward, Single: false, OASReceived: true, Income: income,
		})
		if err != nil {
			t.Fatalf("GISAnnual: %v", err)
		}
		return got
	}

	withinDollar(t, combined(0), 12*667.41)
	withinDollar(t, combined(6000), 12*522.41)
	withinDollarMonthly(t, combined(8736), 436.74)
	withinDollarMonthly(t, combined(29664), 0.74)
	if got := combined(29712); got != 0 {
		t.Fatalf("GIS at the published couple cut-off = %v, want 0", got)
	}

	// The combined test is what decides the claim: each spouse with $15,000
	// would qualify alone, but the couple's $30,000 is over the couple cut-off.
	alone, err := GISAnnual(GISInput{
		Year: 2026, Forward: forward, Single: true, OASReceived: true, Income: 15000,
	})
	if err != nil {
		t.Fatalf("GISAnnual: %v", err)
	}
	if alone <= 0 {
		t.Fatalf("single GIS at $15,000 = %v, want positive", alone)
	}
	together, err := GISAnnual(GISInput{
		Year: 2026, Forward: forward, Single: false, OASReceived: true,
		Income: 15000, PartnerIncome: 15000,
	})
	if err != nil {
		t.Fatalf("GISAnnual: %v", err)
	}
	if together != 0 {
		t.Fatalf("couple GIS at $30,000 combined = %v, want 0", together)
	}
}

// TestGIS_NoCutoffCliff covers the Done-when item that benefits approach zero
// according to the implemented schedule: the benefit is non-increasing across
// the income range and steps by no more than the schedule's 75% marginal rate
// near the cut-off, where the old formula fell by thousands of dollars.
func TestGIS_NoCutoffCliff(t *testing.T) {
	single := func(income float64) float64 {
		t.Helper()
		got, err := GISAnnual(GISInput{
			Year: 2026, Forward: forward, Single: true, OASReceived: true, Income: income,
		})
		if err != nil {
			t.Fatalf("GISAnnual: %v", err)
		}
		return got
	}

	const step = 250.0
	prev := single(0)
	for income := step; income <= 30000; income += step {
		got := single(income)
		if got > prev {
			t.Fatalf("GIS at %v = %v rose above %v at the previous step", income, got, prev)
		}
		// At the steepest band a step of $250 of income can only cut the
		// annual benefit by 0.75 x $250.
		if drop := prev - got; drop > 0.75*step+1e-9 {
			t.Fatalf("GIS dropped %v between %v and %v, want at most %v", drop, income-step, income, 0.75*step)
		}
		prev = got
	}
	if got := single(22487); got > 12 {
		t.Fatalf("GIS one dollar below the cut-off = %v, want within $1/month of nil", got)
	}
}

// TestGIS_EmploymentExemption covers the Done-when item that the
// employment-income exemption is applied when relevant: the first $5,000 of
// employment income is excluded from the test, plus half of the next $10,000.
func TestGIS_EmploymentExemption(t *testing.T) {
	single := func(income, employment float64) float64 {
		t.Helper()
		got, err := GISAnnual(GISInput{
			Year: 2026, Forward: forward, Single: true, OASReceived: true,
			Income: income, EmploymentIncome: employment,
		})
		if err != nil {
			t.Fatalf("GISAnnual: %v", err)
		}
		return got
	}

	// The first $5,000 of employment income does not reduce the supplement.
	withinDollar(t, single(4000, 4000), 12*1108.74)
	// Half of the next $10,000 is exempt: $8,000 of employment income leaves
	// $1,500 in the test.
	withinDollar(t, single(8000, 8000), 12*1108.74-0.5*1500)
	// The same income with no employment component gets no exemption.
	withinDollar(t, single(8000, 0), 12*1108.74-(0.5*2040+0.75*(8000-2040)))
	// The exemption is capped at $10,000 of employment income.
	withinDollar(t, single(20000, 20000), 12*1108.74-(0.5*2040+0.75*(10000-2040)))

	// Each spouse claims the exemption on their own employment income before
	// the couple's combined test.
	couple, err := GISAnnual(GISInput{
		Year: 2026, Forward: forward, Single: false, OASReceived: true,
		Income: 8000, EmploymentIncome: 8000,
		PartnerIncome: 8000, PartnerEmploymentIncome: 8000,
	})
	if err != nil {
		t.Fatalf("GISAnnual: %v", err)
	}
	withinDollar(t, couple, 12*667.41-0.25*3000)
}

// TestGIS_IndexesWithCPI covers the annual evaluation in a later projection
// year: the maximums, the reduction bands, and the cut-off all forward-index
// at CPI.
func TestGIS_IndexesWithCPI(t *testing.T) {
	single := func(income float64) float64 {
		t.Helper()
		got, err := GISAnnual(GISInput{
			Year: 2036, Forward: forward, Single: true, OASReceived: true, Income: income,
		})
		if err != nil {
			t.Fatalf("GISAnnual: %v", err)
		}
		return got
	}
	cpi10 := math.Pow(1.021, 10)

	withinDollar(t, single(0), 12*1108.74*cpi10)
	// The first band's boundary indexes too: it still ends at 50% reduction.
	withinDollar(t, single(2040*cpi10), 12*1108.74*cpi10-0.5*2040*cpi10)

	atThreshold := 22488 * cpi10
	if got := single(atThreshold); got != 0 {
		t.Fatalf("GIS at the indexed 2036 cut-off = %v, want 0", got)
	}
}

// TestGIS_ZeroForTypicalHouseholdEveryYear covers the Done-when item that GIS
// is evaluated in every projection year: the annual evaluation runs for each
// year of the horizon and returns a figure rather than being skipped once the
// household is comfortably above the income test.
func TestGIS_ZeroForTypicalHouseholdEveryYear(t *testing.T) {
	for year := 2026; year <= 2076; year++ {
		for _, single := range []bool{false, true} {
			got, err := GISAnnual(GISInput{
				Year: year, Forward: forward, Single: single, OASReceived: true,
				Income: 120000, PartnerIncome: 120000,
			})
			if err != nil {
				t.Fatalf("GISAnnual(%d, single=%v): %v", year, single, err)
			}
			if got != 0 {
				t.Fatalf("GIS(%d, single=%v) = %v, want 0 for a high-income household", year, single, got)
			}
		}
	}
}

// TestGIS_ReappearsForLowIncomeSurvivor covers the Done-when item's reason
// for evaluating GIS every year: a low-income survivor brings it back above
// zero, on the single schedule once the other spouse has died.
func TestGIS_ReappearsForLowIncomeSurvivor(t *testing.T) {
	gis := func(single bool, income float64) float64 {
		t.Helper()
		got, err := GISAnnual(GISInput{
			Year: 2026, Forward: forward, Single: single, OASReceived: true, Income: income,
		})
		if err != nil {
			t.Fatalf("GISAnnual: %v", err)
		}
		return got
	}

	withinDollar(t, gis(true, 0), 12*1108.74)
	withinDollar(t, gis(true, 10000), 12*1108.74-(0.5*2040+0.75*(10000-2040)))
	withinDollar(t, gis(true, 22488), 0)
	withinDollar(t, gis(true, 30000), 0)
	withinDollar(t, gis(false, 0), 12*667.41)
	withinDollar(t, gis(false, 10000), 12*667.41-(0.25*4080+0.375*(8736-4080)+0.25*(10000-8736)))
	withinDollar(t, gis(false, 50000), 0)
}
