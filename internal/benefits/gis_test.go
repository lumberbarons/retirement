package benefits

import (
	"math"
	"testing"
)

// TestGIS_ZeroForTypicalHouseholdEveryYear covers the Done-when item that GIS
// is evaluated in every projection year, including the years where it returns
// zero: the annual evaluation runs for each year of the horizon and returns a
// figure rather than being skipped once the household is comfortably above
// the income test.
func TestGIS_ZeroForTypicalHouseholdEveryYear(t *testing.T) {
	for year := 2026; year <= 2076; year++ {
		for _, single := range []bool{false, true} {
			got, err := GISAnnual(GISInput{Year: year, Forward: forward, Single: single, OtherIncome: 120000})
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
// zero. The single rate is reduced at $0.50 per $1 of other income and
// reaches nil at the published cut-off; the spouse-of-pensioner rate applies
// while both spouses are alive.
func TestGIS_ReappearsForLowIncomeSurvivor(t *testing.T) {
	gis := func(single bool, income float64) float64 {
		t.Helper()
		got, err := GISAnnual(GISInput{Year: 2026, Forward: forward, Single: single, OtherIncome: income})
		if err != nil {
			t.Fatalf("GISAnnual: %v", err)
		}
		return got
	}

	withinDollar(t, gis(true, 0), 12*1108.74)
	withinDollar(t, gis(true, 10000), 12*1108.74-0.5*10000)
	withinDollar(t, gis(true, 22488), 0)
	withinDollar(t, gis(true, 30000), 0)
	withinDollar(t, gis(false, 0), 12*667.41)
	withinDollar(t, gis(false, 10000), 12*667.41-0.5*10000)
	withinDollar(t, gis(false, 50000), 0)
}

// TestGIS_IndexesWithCPI covers the annual evaluation in a later projection
// year: the maximums and the single cut-off all forward-index at CPI.
func TestGIS_IndexesWithCPI(t *testing.T) {
	got, err := GISAnnual(GISInput{Year: 2036, Forward: forward, Single: true, OtherIncome: 0})
	if err != nil {
		t.Fatalf("GISAnnual: %v", err)
	}
	withinDollar(t, got, 12*1108.74*math.Pow(1.021, 10))

	// The cut-off indexes too: nominal income equal to the indexed 2036
	// cut-off leaves no GIS.
	atThreshold := 22488 * math.Pow(1.021, 10)
	got, err = GISAnnual(GISInput{Year: 2036, Forward: forward, Single: true, OtherIncome: atThreshold})
	if err != nil {
		t.Fatalf("GISAnnual: %v", err)
	}
	if got != 0 {
		t.Fatalf("GIS at the indexed 2036 cut-off = %v, want 0", got)
	}
}
